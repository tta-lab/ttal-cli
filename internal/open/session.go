package open

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// Package-level overrides for test injection.
var (
	exportTaskFn    = taskwarrior.ExportTask
	sessionExistsFn = tmux.SessionExists
	windowExistsFn  = tmux.WindowExists
	attachFn        = attachToSession
	attachWindowFn  = attachToWindowSelect
	resolveTargetFn = worker.ResolveTmuxTarget
	configLoaderFn  = func() (*config.Config, error) {
		return config.Load()
	}
)

// Session attaches to the tmux session associated with a task.
// Checks worker session first, then falls back to owner agent session.
func Session(uuid string) error {
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}

	task, err := exportTaskFn(uuid)
	if err != nil {
		return err
	}

	// First, try to resolve the worker target (manager session + agent-name window).
	// A worker window exists → select that window and attach to the manager session.
	if task.Owner != "" {
		wt, err := resolveTargetFn(task)
		if err == nil && windowExistsFn(wt.Session, wt.Window) {
			return attachWindowFn(wt.Session, wt.Window)
		}
	}

	// Legacy fallback: check for old w-* session naming.
	sessionName := task.SessionName()
	if sessionExistsFn(sessionName) {
		return attachFn(sessionName)
	}

	// Fall back to owner agent session (for plan-review, manager tasks).
	if task.Owner != "" {
		cfg, err := configLoaderFn()
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not load config for owner session lookup: %w", err)
		}
		if cfg != nil {
			ownerSession := config.AgentSessionName(task.Owner)
			if sessionExistsFn(ownerSession) {
				return attachFn(ownerSession)
			}
		}
	}

	return fmt.Errorf("no worker window or agent session for this task\n\n"+
		"  To spawn a worker:\n"+
		"  ttal go %s", uuid)
}

func attachToSession(sessionName string) error {
	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionName,
	}, os.Environ())
}

// attachToWindowSelect first selects the target window in the session, then
// attaches to the session. This is safer than relying on session:window attach
// syntax, which is not consistently supported across tmux versions.
func attachToWindowSelect(session, window string) error {
	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}

	// First select the target window
	selectCmd := exec.Command(tmuxBin, "select-window", "-t", session+":"+window)
	if err := selectCmd.Run(); err != nil {
		return fmt.Errorf("failed to select window %s:%s: %w", session, window, err)
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", session,
	}, os.Environ())
}
