package cmd

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// workerWindowName returns the tmux window name for the worker agent based on task tags.
// Falls back to the default worker agent name if the pipeline config is unavailable
// or no worker stage matches the tags.
func workerWindowName(taskTags []string) string {
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		return worker.CoderAgentName
	}
	if name := pipelineCfg.WorkerAgentName(taskTags); name != "" {
		return name
	}
	return worker.CoderAgentName
}

// resolveTaskTags returns the task tags for the current session, or nil if unavailable.
func resolveTaskTags() []string {
	jobID := os.Getenv("TTAL_JOB_ID")
	if jobID == "" {
		return nil
	}
	task, err := taskwarrior.ExportTaskByHexID(jobID, "pending")
	if err != nil {
		task, err = taskwarrior.ExportTaskByHexID(jobID, "completed")
	}
	if err != nil || task == nil {
		return nil
	}
	return task.Tags
}

// resolveReviewerWindow returns the reviewer agent name (= window name) for a
// given pipeline assignee role and task tags. Falls back to fallback.
func resolveReviewerWindow(taskTags []string, assigneeRole, fallback string) string {
	return resolveReviewerWindowForDir(config.DefaultConfigDir(), taskTags, assigneeRole, fallback)
}

// resolveReviewerWindowForDir is the testable form of resolveReviewerWindow,
// accepting an explicit config directory (matching the handleCloseWindowWithConfigDir pattern).
func resolveReviewerWindowForDir(configDir string, taskTags []string, assigneeRole, fallback string) string {
	pipelineCfg, err := pipeline.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load pipelines.toml — falling back to %s: %v\n", fallback, err)
		return fallback
	}
	if name := pipelineCfg.ReviewerForStage(taskTags, assigneeRole); name != "" {
		return name
	}
	return fallback
}
