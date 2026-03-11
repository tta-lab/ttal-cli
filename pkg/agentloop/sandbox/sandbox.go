package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

const maxOutputBytes = 64 * 1024 // 64KB output truncation

// Sandbox wraps bubblewrap for isolated command execution.
type Sandbox struct {
	BwrapPath        string
	Timeout          time.Duration
	MemoryMB         int
	AllowUnsandboxed bool // if false (default), fail hard when bwrap is unavailable
}

// Seconds returns a duration from a seconds count.
func Seconds(s int) time.Duration {
	return time.Duration(s) * time.Second
}

// ExecConfig holds per-execution sandbox settings.
type ExecConfig struct {
	Env       []string // Extra env vars passed to the sandboxed process
	MountDirs []Mount  // Additional read-only bind mounts
}

// Mount represents a filesystem mount inside the sandbox.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// Exec runs a bash command inside the bubblewrap sandbox.
// If bwrap is unavailable and AllowUnsandboxed is false, it returns an error.
// If AllowUnsandboxed is true (dev mode), it falls back to direct exec.
func (s *Sandbox) Exec(
	ctx context.Context, command string, cfg *ExecConfig,
) (stdout, stderr string, exitCode int, err error) {
	if !s.IsAvailable() {
		if !s.AllowUnsandboxed {
			return "", "", -1, fmt.Errorf(
				"bwrap not found at %q — set AllowUnsandboxed=true for local dev without bubblewrap",
				s.BwrapPath,
			)
		}
		return s.execDirect(ctx, command, cfg)
	}

	timeout := s.effectiveTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := s.buildArgs(command, cfg)
	cmd := exec.CommandContext(ctx, s.BwrapPath, args...)
	cmd.Env = buildEnv(cfg)

	return runCmd(ctx, cmd)
}

// IsAvailable checks whether bwrap is available at the configured path.
func (s *Sandbox) IsAvailable() bool {
	_, err := exec.LookPath(s.BwrapPath)
	return err == nil
}

// execDirect runs a command without bwrap (dev-only, requires AllowUnsandboxed).
func (s *Sandbox) execDirect(
	ctx context.Context, command string, cfg *ExecConfig,
) (stdout, stderr string, exitCode int, err error) {
	timeout := s.effectiveTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Env = buildEnv(cfg)

	return runCmd(ctx, cmd)
}

// runCmd executes a prepared command and returns output, exit code, and errors.
// It distinguishes between context cancellation (timeout) and other exec errors.
func runCmd(ctx context.Context, cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()

	stdoutStr := truncate(stdoutBuf.String(), maxOutputBytes)
	stderrStr := truncate(stderrBuf.String(), maxOutputBytes)

	if ctx.Err() != nil {
		return stdoutStr, stderrStr, -1, ctx.Err()
	}

	// Distinguish successful exit (including non-zero) from exec infrastructure failure.
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdoutStr, stderrStr, exitErr.ExitCode(), nil
	}
	if runErr != nil {
		return stdoutStr, stderrStr, -1, fmt.Errorf("exec failed: %w", runErr)
	}

	return stdoutStr, stderrStr, 0, nil
}

func (s *Sandbox) buildArgs(command string, cfg *ExecConfig) []string {
	args := []string{
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--tmpfs", "/tmp",
		"--tmpfs", "/home/agent",
		"--unshare-all",
		"--share-net",
		"--dev", "/dev",
		"--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf",
		"--ro-bind", "/etc/ssl/certs", "/etc/ssl/certs",
		"--ro-bind", "/etc/hosts", "/etc/hosts",
		"--die-with-parent",
	}

	if runtime.GOOS == "linux" {
		args = append(args,
			"--ro-bind", "/lib", "/lib",
			"--symlink", "/usr/lib64", "/lib64",
		)
	}

	if cfg != nil {
		for _, m := range cfg.MountDirs {
			if m.ReadOnly {
				args = append(args, "--ro-bind", m.Source, m.Target)
			} else {
				args = append(args, "--bind", m.Source, m.Target)
			}
		}
	}

	args = append(args, "--", "bash", "-c", command)
	return args
}

func buildEnv(cfg *ExecConfig) []string {
	base := []string{
		"PATH=/usr/bin:/usr/local/bin:/bin",
		"HOME=/home/agent",
		"TERM=dumb",
	}
	if cfg != nil {
		base = append(base, cfg.Env...)
	}
	return base
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n[output truncated]"
}

func (s *Sandbox) effectiveTimeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 30 * time.Second
}
