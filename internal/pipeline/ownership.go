package pipeline

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// exportTasksByFilterFn is the function used to query taskwarrior.
// Package-level var for test injection.
var exportTasksByFilterFn = taskwarrior.ExportTasksByFilter

// ActiveTasksByOwner returns pending+active tasks owned by the given agent,
// EXCLUDING tasks currently in a worker:true stage. Worker-stage tasks are driven
// by a worker session, not the manager agent, so they must not count against the
// owner's busy quota.
func ActiveTasksByOwner(cfg *Config, owner string) ([]taskwarrior.Task, error) {
	tasks, err := exportTasksByFilterFn("status:pending", "+ACTIVE", "owner:"+owner)
	if err != nil {
		return nil, fmt.Errorf("export tasks for owner %q: %w", owner, err)
	}

	// Filter out tasks in a worker stage.
	var filtered []taskwarrior.Task
	for _, task := range tasks {
		_, p, err := cfg.MatchPipeline(task.Tags)
		if err != nil || p == nil {
			// No pipeline match — can't classify; include to be safe.
			filtered = append(filtered, task)
			continue
		}
		_, stage, _ := p.CurrentStage(task.Tags)
		if stage != nil && stage.IsWorker() {
			continue
		}
		filtered = append(filtered, task)
	}

	return filtered, nil
}

// CountActiveTasksByOwner is a thin wrapper returning the count of non-worker
// active tasks owned by the given agent.
func CountActiveTasksByOwner(cfg *Config, owner string) (int, error) {
	tasks, err := ActiveTasksByOwner(cfg, owner)
	if err != nil {
		return 0, err
	}
	return len(tasks), nil
}
