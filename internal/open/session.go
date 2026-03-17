package open

import (
	"fmt"
	"os"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/flicktask"
)

// Session attaches to the tmux session associated with a task.
func Session(uuid string) error {
	if err := flicktask.ValidateID(uuid); err != nil {
		return err
	}

	task, err := flicktask.ExportTask(uuid)
	if err != nil {
		return err
	}

	sessionID := task.SessionName()
	if task.Branch == "" {
		return fmt.Errorf("no worker session assigned to this task\n\n"+
			"  To spawn a worker for this task:\n"+
			"  ttal task execute %s", uuid)
	}

	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionID,
	}, os.Environ())
}
