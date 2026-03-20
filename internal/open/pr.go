package open

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
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
		sessionName := task.SessionName()
		if tmux.SessionExists(sessionName) {
			msg += fmt.Sprintf("\n\n  Worker session '%s' is active but hasn't created a PR yet.", sessionName)
		} else {
			msg += "\n\n  No worker is currently working on this task."
		}
		return fmt.Errorf("%s", msg)
	}

	projectPath, err := project.ResolveProjectPathOrError(task.Project)
	if err != nil {
		return err
	}

	prInfo, err := taskwarrior.ParsePRID(task.PRID)
	if err != nil {
		return fmt.Errorf("invalid pr_id: %w", err)
	}

	repoInfo, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return fmt.Errorf("cannot determine repo: %w", err)
	}

	prURL := repoInfo.PRURL(strconv.FormatInt(prInfo.Index, 10))
	fmt.Printf("Opening PR #%d: %s\n", prInfo.Index, prURL)
	fmt.Printf("Repository: %s/%s (%s)\n", repoInfo.Owner, repoInfo.Repo, repoInfo.Provider)

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
