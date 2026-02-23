package pr

import (
	"fmt"
	"os"

	forgejoapi "codeberg.org/clawteam/ttal-cli/internal/forgejo"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

// Context holds the resolved PR context from the current worker session.
type Context struct {
	Task  *taskwarrior.Task
	Owner string
	Repo  string
}

// ResolveContext resolves the full PR context from the current worker session.
// Context is auto-resolved from TTAL_JOB_ID (task UUID prefix).
func ResolveContext() (*Context, error) {
	task, err := resolveTask()
	if err != nil {
		return nil, err
	}

	if task.ProjectPath == "" {
		return nil, fmt.Errorf("task has no project_path UDA set")
	}

	owner, repo, err := forgejoapi.ParseRepoInfo(task.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("cannot determine repo from %s: %w", task.ProjectPath, err)
	}

	return &Context{Task: task, Owner: owner, Repo: repo}, nil
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
