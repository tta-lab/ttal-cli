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
// Falls back to --task flag if not in a worker session.
func ResolveContext(taskUUID string) (*Context, error) {
	task, err := resolveTask(taskUUID)
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

// resolveTask finds the task either from TTAL_JOB_ID or an explicit UUID.
func resolveTask(taskUUID string) (*taskwarrior.Task, error) {
	if taskUUID != "" {
		if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
			return nil, err
		}
		return taskwarrior.ExportTask(taskUUID)
	}

	// Auto-resolve from job ID (task UUID[:8])
	jobID := os.Getenv("TTAL_JOB_ID")
	if jobID == "" {
		return nil, fmt.Errorf("not in a worker session — provide --task <uuid> explicitly")
	}

	// Try pending (active worker), then completed (just finished)
	task, err := taskwarrior.ExportTaskBySessionID(jobID, "pending")
	if err != nil {
		task, err = taskwarrior.ExportTaskBySessionID(jobID, "completed")
		if err != nil {
			return nil, fmt.Errorf("no task found for job ID %q", jobID)
		}
	}

	return task, nil
}
