package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SeatbeltSandbox executes commands using macOS seatbelt (sandbox-exec).
// Used on macOS for kernel-level sandboxing with deny-default filesystem policy.
type SeatbeltSandbox struct {
	Timeout time.Duration
}

// Exec runs a bash command inside the seatbelt sandbox.
func (s *SeatbeltSandbox) Exec(ctx context.Context, command string, cfg *ExecConfig) (stdout, stderr string, exitCode int, err error) {
	timeout := effectiveTimeout(s.Timeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a temp HOME dir — always clean up, even on error or timeout.
	homeDir, err := os.MkdirTemp("/tmp", "ttal-agent-")
	if err != nil {
		return "", "", -1, fmt.Errorf("create temp home: %w", err)
	}
	defer os.RemoveAll(homeDir)

	policy, dParams, err := buildPolicy(cfg)
	if err != nil {
		return "", "", -1, fmt.Errorf("build seatbelt policy: %w", err)
	}

	args := []string{"-p", policy}
	args = append(args, dParams...)
	args = append(args, "--", "bash", "-c", command)

	cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", args...)
	cmd.Env = buildEnv(cfg, homeDir)

	return runCmd(ctx, cmd)
}

// IsAvailable checks whether sandbox-exec is present on the system.
func (s *SeatbeltSandbox) IsAvailable() bool {
	_, err := os.Stat("/usr/bin/sandbox-exec")
	return err == nil
}
