package open

import (
	"fmt"
	"os"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// Session attaches to the zellij session associated with a task.
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

	zellijBin, err := lookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found in PATH")
	}

	return syscall.Exec(zellijBin, []string{
		"zellij", "--data-dir", zellij.DataDir(), "attach", sessionID,
	}, os.Environ())
}
