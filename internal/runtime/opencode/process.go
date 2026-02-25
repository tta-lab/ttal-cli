package opencode

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// process manages the `opencode serve` child process.
type process struct {
	cmd     *exec.Cmd
	port    int
	workDir string
	env     []string
}

// start spawns `opencode serve --port N --hostname 127.0.0.1`.
func (p *process) start(ctx context.Context) error {
	p.cmd = exec.Command("opencode", "serve",
		"--hostname", "127.0.0.1",
		"--port", fmt.Sprintf("%d", p.port),
	)
	p.cmd.Dir = p.workDir
	p.cmd.Env = append(os.Environ(), p.env...)
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start opencode serve: %w", err)
	}

	if err := p.waitReady(ctx, 15*time.Second); err != nil {
		p.stop()
		return fmt.Errorf("opencode serve not ready: %w", err)
	}

	return nil
}

// waitReady polls the server until it responds or timeout.
func (p *process) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/doc", p.port)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := http.Get(url) //nolint:gosec,noctx // localhost polling
		if err == nil {
			resp.Body.Close()
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
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
}

// isRunning checks if the process is still alive.
func (p *process) isRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	return p.cmd.ProcessState == nil
}
