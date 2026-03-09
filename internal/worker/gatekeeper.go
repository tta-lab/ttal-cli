package worker

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// GatekeeperConfig holds configuration for the gatekeeper process.
type GatekeeperConfig struct {
	TaskFile string
	Command  []string
}

// Gatekeeper runs a child command with deadman's switch behavior:
// - Reads task file and appends content to child args
// - Starts child in its own process group
// - Monitors for parent death (orphaning)
// - Handles signals (SIGTERM, SIGINT, SIGHUP) by killing child
func Gatekeeper(cfg GatekeeperConfig) int {
	if len(cfg.Command) == 0 {
		fmt.Fprintln(os.Stderr, "gatekeeper: no command specified")
		return 1
	}

	args := make([]string, len(cfg.Command))
	copy(args, cfg.Command)

	// Append task file content to command args
	if cfg.TaskFile != "" {
		content, err := os.ReadFile(cfg.TaskFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gatekeeper: failed to read task file: %v\n", err)
			return 1
		}
		args = append(args, strings.TrimSpace(string(content)))
	}

	// Set up environment
	env := os.Environ()
	if os.Getenv("TERM") == "" {
		env = append(env, "TERM=xterm-256color")
	}
	env = setEnv(env, "FORCE_COLOR", "1")

	// Start child in its own process group
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true,
		Foreground: true,
		Ctty:       int(os.Stdin.Fd()),
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "gatekeeper: failed to start child: %v\n", err)
		return 1
	}

	originalPPID := os.Getppid()

	var (
		stopOnce sync.Once
		stopCh   = make(chan struct{})
	)

	cleanup := func() {
		stopOnce.Do(func() {
			close(stopCh)
			gracefulStopChild(cmd.Process.Pid)
		})
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		select {
		case <-sigCh:
			cleanup()
		case <-stopCh:
		}
	}()

	// Monitor for parent death (orphaning). Detects re-parenting to any
	// process (not just PID 1), which handles subreaper environments.
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-time.After(time.Second):
				if os.Getppid() != originalPPID {
					cleanup()
					return
				}
			}
		}
	}()

	// Wait for child to exit
	err := cmd.Wait()
	stopOnce.Do(func() { close(stopCh) })

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "gatekeeper: child wait error: %v\n", err)
		return 1
	}
	return 0
}

// gracefulStopChild sends SIGINT to the process group for graceful exit,
// waits up to 5s, then force-kills with SIGKILL if still running.
func gracefulStopChild(pid int) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		if err != syscall.ESRCH {
			fmt.Fprintf(os.Stderr, "gatekeeper: getpgid(%d) failed: %v\n", pid, err)
		}
		return
	}

	// Send SIGINT for graceful exit (Claude Code saves conversation on SIGINT)
	if err := syscall.Kill(-pgid, syscall.SIGINT); err != nil && err != syscall.ESRCH {
		fmt.Fprintf(os.Stderr, "gatekeeper: SIGINT to pgid %d failed: %v\n", pgid, err)
	}

	// Wait up to 5s for exit
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-pgid, 0); err == syscall.ESRCH {
			return // process gone
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Force kill if still running
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		fmt.Fprintf(os.Stderr, "gatekeeper: SIGKILL to pgid %d failed: %v\n", pgid, err)
	}
}

// setEnv sets or replaces an environment variable in a slice.
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
