package open

import (
	"fmt"
	"os"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// Session attaches to the tmux session associated with a task.
func Session(uuid string) error {
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		return err
	}

	sessionName := task.SessionName()
	if !tmux.SessionExists(sessionName) {
		return fmt.Errorf("no worker session assigned to this task\n\n"+
			"  To spawn a worker for this task:\n"+
			"  ttal task advance %s", uuid)
	}

	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionName,
	}, os.Environ())
}
