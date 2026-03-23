package cmd

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// coderWindowName is the fixed window name for coder sessions.
const coderWindowName = "coder"

// resolveTaskTags returns the task tags for the current session, or nil if unavailable.
func resolveTaskTags() []string {
	jobID := os.Getenv("TTAL_JOB_ID")
	if jobID == "" {
		return nil
	}
	task, err := taskwarrior.ExportTaskBySessionID(jobID, "pending")
	if err != nil {
		task, err = taskwarrior.ExportTaskBySessionID(jobID, "completed")
	}
	if err != nil || task == nil {
		return nil
	}
	return task.Tags
}

// resolveReviewerWindow returns the reviewer agent name (= window name) for a
// given pipeline assignee role and task tags. Falls back to fallback.
func resolveReviewerWindow(taskTags []string, assigneeRole, fallback string) string {
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load pipelines.toml — falling back to %s: %v\n", fallback, err)
		return fallback
	}
	if name := pipelineCfg.ReviewerForStage(taskTags, assigneeRole); name != "" {
		return name
	}
	return fallback
}
