package pr

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
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
	task, err := resolveTask()
	if err != nil {
		return nil, err
	}

	if task.ProjectPath == "" {
		return nil, fmt.Errorf("task has no project_path UDA set")
	}

	info, err := gitprovider.DetectProvider(task.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("cannot determine repo from %s: %w", task.ProjectPath, err)
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

// resolveTask finds the task from TTAL_JOB_ID.
func resolveTask() (*taskwarrior.Task, error) {
	jobID := os.Getenv("TTAL_JOB_ID")
	if jobID == "" {
		return nil, fmt.Errorf("not in a worker session — run this from a worker session")
	}

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
