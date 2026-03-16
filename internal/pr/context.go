package pr

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

type Context struct {
	Task     *taskwarrior.Task
	Owner    string
	Repo     string
	Provider gitprovider.Provider
	Info     *gitprovider.RepoInfo
}

func ResolveContext() (*Context, error) {
	jobID := os.Getenv("TTAL_JOB_ID")
	if jobID == "" {
		return resolveFromCwd()
	}
	return resolveFromTask(jobID)
}

func resolveFromTask(jobID string) (*Context, error) {
	task, err := resolveTask(jobID)
	if err != nil {
		return nil, err
	}

	projectPath, err := project.ResolveProjectPathOrError(task.Project)
	if err != nil {
		return nil, err
	}

	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return nil, fmt.Errorf("cannot determine repo from %s: %w", projectPath, err)
	}

	provider, err := gitprovider.NewProvider(info)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider client: %w", err)
	}

	return &Context{
		Task:     task,
		Owner:    info.Owner,
		Repo:     info.Repo,
		Provider: provider,
		Info:     info,
	}, nil
}

func resolveFromCwd() (*Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot determine working directory: %w", err)
	}

	info, err := gitprovider.DetectProvider(cwd)
	if err != nil {
		return nil, fmt.Errorf("not in a git repo with a recognized remote: %w", err)
	}

	provider, err := gitprovider.NewProvider(info)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider client: %w", err)
	}

	return &Context{
		Task:     &taskwarrior.Task{},
		Owner:    info.Owner,
		Repo:     info.Repo,
		Provider: provider,
		Info:     info,
	}, nil
}

// resolveTask finds the task from TTAL_JOB_ID.
func resolveTask(jobID string) (*taskwarrior.Task, error) {
	// Try pending (active worker), then completed (just finished)
	task, err := taskwarrior.ExportTaskBySessionID(jobID, "pending")
	if err != nil {
		task, err = taskwarrior.ExportTaskBySessionID(jobID, "completed")
		if err != nil {
			return nil, fmt.Errorf("no task found for job ID %q: %w", jobID, err)
		}
	}

	return task, nil
}
