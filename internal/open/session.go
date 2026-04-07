package open

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// Package-level overrides for test injection.
var (
	exportTaskFn    = taskwarrior.ExportTask
	sessionExistsFn = tmux.SessionExists
	attachFn        = attachToSession
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

	// Try worker session first.
	sessionName := task.SessionName()
	if sessionExistsFn(sessionName) {
		return attachFn(sessionName)
	}

	// Fall back to owner agent session if task has owner UDA set.
	// Worker-stage tasks have no owner written (advance.go writes owner only
	// when !stage.IsWorker()), so this branch is skipped for worker tasks.
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

	return fmt.Errorf("no worker or agent session for this task\n\n"+
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
