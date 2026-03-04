package codex

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// process manages the `codex app-server` child process.
type process struct {
	cmd           *exec.Cmd
	port          int
	workDir       string
	env           []string
	writableRoots []string
	exited        chan struct{} // closed when cmd.Wait() returns
}

// start spawns `codex app-server --listen ws://127.0.0.1:<port>`.
func (p *process) start(ctx context.Context) error {
	listenAddr := fmt.Sprintf("ws://127.0.0.1:%d", p.port)

	args := []string{"app-server", "--listen", listenAddr}
	if len(p.writableRoots) > 0 {
		quoted := make([]string, len(p.writableRoots))
		for i, r := range p.writableRoots {
			quoted[i] = fmt.Sprintf("%q", r)
		}
		roots := fmt.Sprintf("sandbox_workspace_write.writable_roots=[%s]", strings.Join(quoted, ", "))
		args = append(args, "-c", roots)
	}

	p.cmd = exec.Command("codex", args...)
	p.cmd.Dir = p.workDir
	p.cmd.Env = append(os.Environ(), p.env...)
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start codex app-server: %w", err)
	}

	// Reap the child process to avoid zombies and populate ProcessState.
	p.exited = make(chan struct{})
	go func() {
		if err := p.cmd.Wait(); err != nil {
			log.Printf("[codex] app-server exited with error: %v", err)
		}
		close(p.exited)
	}()

	if err := p.waitReady(ctx, 15*time.Second); err != nil {
		p.stop()
		return fmt.Errorf("codex app-server not ready: %w", err)
	}

	return nil
}

// waitReady polls the server by attempting TCP connect until ready.
func (p *process) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", p.port)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

// stop terminates the process gracefully with SIGINT, falling back to SIGKILL.
func (p *process) stop() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	select {
	case <-p.exited:
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		select {
		case <-p.exited:
		case <-time.After(3 * time.Second):
			log.Printf("[codex] process did not exit after SIGKILL, giving up")
		}
	}
}

// isRunning checks if the process is still alive.
func (p *process) isRunning() bool {
	if p.exited == nil {
		return false
	}
	select {
	case <-p.exited:
		return false
	default:
		return true
	}
}
