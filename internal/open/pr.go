package open

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/guion-opensource/ttal-cli/internal/taskwarrior"
)

var remoteURLPattern = regexp.MustCompile(`(?:ssh://[^/]*/|git@[^:]+:)([^/]+)/([^/]+?)(?:\.git)?$`)

// PR opens the Forgejo PR URL for a task in the default browser.
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
		if task.SessionName != "" {
			msg += fmt.Sprintf("\n\n  Worker session '%s' is active but hasn't created a PR yet.", task.SessionName)
		} else {
			msg += "\n\n  No worker is currently working on this task."
		}
		return fmt.Errorf("%s", msg)
	}

	if task.ProjectPath == "" {
		return fmt.Errorf("task has PR #%s but no project_path UDA\n\n  Unable to construct PR URL.", task.PRID)
	}

	owner, repo := resolveOwnerRepo(task.ProjectPath)

	forgejoURL := os.Getenv("FORGEJO_URL")
	if forgejoURL == "" {
		forgejoURL = "https://git.guion.io"
	}

	prURL := fmt.Sprintf("%s/%s/%s/pulls/%s", forgejoURL, owner, repo, task.PRID)
	fmt.Printf("Opening PR #%s: %s\n", task.PRID, prURL)
	fmt.Printf("Repository: %s/%s\n", owner, repo)

	return openBrowser(prURL)
}

func resolveOwnerRepo(projectPath string) (owner, repo string) {
	repo = filepath.Base(projectPath)
	owner = os.Getenv("FORGEJO_DEFAULT_OWNER")
	if owner == "" {
		owner = "neil"
	}

	remoteURL, err := gitRemoteURL(projectPath)
	if err != nil {
		return owner, repo
	}

	matches := remoteURLPattern.FindStringSubmatch(remoteURL)
	if len(matches) >= 3 {
		owner = matches[1]
		repo = strings.TrimSuffix(matches[2], ".git")
	}

	return owner, repo
}

func gitRemoteURL(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
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
