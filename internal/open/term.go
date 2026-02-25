package open

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// Term opens a terminal for the tmux session associated with a task.
// If already inside tmux, it switches the client to the worker session.
// Otherwise, it attaches to the session directly.
func Term(uuid string) error {
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		return err
	}

	sessionName := task.SessionName()
	if task.Branch == "" {
		return fmt.Errorf("no worker session assigned to this task\n\n" +
			"  To spawn a worker for this task:\n" +
			"  ttal worker spawn --task " + uuid + " --project <path> --name <worker-name>")
	}

	if !tmux.SessionExists(sessionName) {
		return fmt.Errorf("tmux session %q not found\n\n"+
			"  The worker session may have been closed.\n"+
			"  Spawn a new worker with:\n"+
			"  ttal worker spawn --task "+uuid+" --project <path> --name <worker-name>", sessionName)
	}

	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	// Inside tmux: switch-client to the worker session (no nesting).
	// Outside tmux: attach-session directly.
	if os.Getenv("TMUX") != "" {
		cmd := exec.Command(tmuxBin, "switch-client", "-t", sessionName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionName,
	}, os.Environ())
}
