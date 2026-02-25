package open

import (
	"fmt"
	"os/exec"
	"runtime"

	"codeberg.org/clawteam/ttal-cli/internal/gitprovider"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

func PR(uuid string) error {
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		return err
	}

	if task.PRID == "" {
		msg := "no PR associated with this task"
		if task.Branch != "" {
			msg += fmt.Sprintf("\n\n  Worker session '%s' is active but hasn't created a PR yet.", task.SessionName())
		} else {
			msg += "\n\n  No worker is currently working on this task."
		}
		return fmt.Errorf("%s", msg)
	}

	if task.ProjectPath == "" {
		return fmt.Errorf("task has PR #%s but no project_path UDA", task.PRID)
	}

	info, err := gitprovider.DetectProvider(task.ProjectPath)
	if err != nil {
		return fmt.Errorf("cannot determine repo: %w", err)
	}

	prURL := info.PRURL(task.PRID)
	fmt.Printf("Opening PR #%s: %s\n", task.PRID, prURL)
	fmt.Printf("Repository: %s/%s (%s)\n", info.Owner, info.Repo, info.Provider)

	return openBrowser(prURL)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform — open this URL manually:\n  %s", url)
	}
	return cmd.Start()
}
