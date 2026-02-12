package open

import (
	"fmt"
	"os"
	"syscall"

	"github.com/guion-opensource/ttal-cli/internal/taskwarrior"
	"github.com/guion-opensource/ttal-cli/internal/zellij"
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

	if task.SessionName == "" {
		return fmt.Errorf("no worker session assigned to this task\n\n" +
			"  To spawn a worker for this task:\n" +
			"  ttal worker spawn --task " + uuid + " --project <path> --name <worker-name>")
	}

	zellijBin, err := lookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found in PATH")
	}

	return syscall.Exec(zellijBin, []string{
		"zellij", "--data-dir", zellij.DataDir(), "attach", task.SessionName,
	}, os.Environ())
}
