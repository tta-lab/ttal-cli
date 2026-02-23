package open

import (
	"fmt"
	"os"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
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

	sessionID := task.SessionName()
	if task.Branch == "" {
		return fmt.Errorf("no worker session assigned to this task\n\n" +
			"  To spawn a worker for this task:\n" +
			"  ttal worker spawn --task " + uuid + " --project <path> --name <worker-name>")
	}

	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionID,
	}, os.Environ())
}
