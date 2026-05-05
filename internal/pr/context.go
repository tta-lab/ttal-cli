package pr

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var (
	taskHexFromCwdFn            = worker.TaskHexFromCwd
	exportTaskByHexIDFn         = taskwarrior.ExportTaskByHexID
	resolveProjectPathOrErrorFn = project.ResolveProjectPathOrError
	detectProviderFn            = gitprovider.DetectProvider
)

type Context struct {
	Task  *taskwarrior.Task
	Owner string
	Repo  string
	Info  *gitprovider.RepoInfo
	Alias string // resolved project alias (from task.Project or cwd path)
}

// ResolveContextWithoutProvider resolves task metadata and git repo info
// without creating an authenticated provider. Used by CLI commands that
// proxy API calls through the daemon.
func ResolveContextWithoutProvider() (*Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot determine working directory: %w", err)
	}
	if hexID := taskHexFromCwdFn(cwd); hexID != "" {
		if task, info, err := resolveTaskInfo(hexID); err == nil {
			return &Context{
				Task:  task,
				Owner: info.Owner,
				Repo:  info.Repo,
				Info:  info,
				Alias: task.Project,
			}, nil
		}
	}
	return resolveFromCwdWithoutProvider()
}

func resolveFromCwdWithoutProvider() (*Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot determine working directory: %w", err)
	}

	info, err := detectProviderFn(cwd)
	if err != nil {
		return nil, fmt.Errorf("not in a git repo with a recognized remote: %w", err)
	}

	return &Context{
		Task:  &taskwarrior.Task{},
		Owner: info.Owner,
		Repo:  info.Repo,
		Info:  info,
		Alias: project.ResolveProjectAlias(cwd),
	}, nil
}

// resolveTaskInfo is shared setup for resolveFromTaskWithoutProvider.
func resolveTaskInfo(jobID string) (*taskwarrior.Task, *gitprovider.RepoInfo, error) {
	task, err := resolveTask(jobID)
	if err != nil {
		return nil, nil, err
	}
	projectPath, err := resolveProjectPathOrErrorFn(task.Project)
	if err != nil {
		return nil, nil, err
	}
	info, err := detectProviderFn(projectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot determine repo from %s: %w", projectPath, err)
	}
	return task, info, nil
}

// resolveTask finds the task from TTAL_JOB_ID.
func resolveTask(jobID string) (*taskwarrior.Task, error) {
	task, err := exportTaskByHexIDFn(jobID, "pending")
	if err != nil {
		task, err = exportTaskByHexIDFn(jobID, "completed")
		if err != nil {
			return nil, fmt.Errorf("no task found for job ID %q: %w", jobID, err)
		}
	}
	return task, nil
}
