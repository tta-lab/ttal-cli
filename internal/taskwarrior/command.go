package taskwarrior

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// Command builds an exec.Cmd for `task` with the active team's TASKRC
// prepended as `rc:<path>`. This ensures all task invocations target
// the correct taskwarrior instance regardless of the calling process's env.
//
// Usage:
//
//	cmd := taskwarrior.Command("status:pending", "export")
//	out, err := cmd.Output()
func Command(args ...string) *exec.Cmd {
	return commandContext(context.Background(), args...)
}

// CommandContext is like Command but with a context for cancellation/timeout.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return commandContext(ctx, args...)
}

func commandContext(ctx context.Context, args ...string) *exec.Cmd {
	return commandContextWithVerbose(ctx, "nothing", args...)
}

func commandContextWithVerbose(ctx context.Context, verbose string, args ...string) *exec.Cmd {
	fullArgs := make([]string, 0, len(args)+2)
	fullArgs = append(fullArgs, "rc.verbose:"+verbose)

	if taskrc := resolveTaskRC(); taskrc != "" {
		fullArgs = append(fullArgs, "rc:"+taskrc)
	}

	fullArgs = append(fullArgs, args...)
	return exec.CommandContext(ctx, "task", fullArgs...)
}

// ResolveDataLocation asks taskwarrior for the actual data.location value,
// respecting the active team's taskrc. Returns the resolved absolute path.
func ResolveDataLocation() (string, error) {
	out, err := Command("_get", "rc.data.location").Output()
	if err != nil {
		return "", fmt.Errorf("failed to query task data.location: %w", err)
	}
	loc := strings.TrimSpace(string(out))
	if loc == "" {
		return "", fmt.Errorf("task data.location is empty")
	}
	return loc, nil
}

// ResolvePowerSyncDBPath asks taskwarrior for the active replica's PowerSync DB
// path (rc.powersync.db_path), expanding ~ to $HOME. The tw fork uses this
// path as its PowerSync replica; watching its WAL file (path + "-wal") is the
// SSOT for taskwarrior change events.
func ResolvePowerSyncDBPath() (string, error) {
	out, err := Command("_get", "rc.powersync.db_path").Output()
	if err != nil {
		return "", fmt.Errorf("failed to query task powersync.db_path: %w", err)
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", fmt.Errorf("task powersync.db_path is empty")
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		p = filepath.Join(home, p[2:])
	}
	return p, nil
}

// resolveTaskRC returns the active team's taskrc path if it differs
// from the system default (~/.taskrc). Returns "" if default or on error.
func resolveTaskRC() string {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not resolve team TASKRC: %v\n", err)
		return ""
	}
	taskrc := cfg.TaskRC
	if taskrc == config.DefaultTaskRC() {
		return ""
	}
	return taskrc
}
