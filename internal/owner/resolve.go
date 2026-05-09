package owner

import (
	"os"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// exportTaskByHexIDFn allows test injection of the task lookup function.
var exportTaskByHexIDFn = taskwarrior.ExportTaskByHexID

// ResolveOwner resolves the owner for the current session.
//
// If TTAL_JOB_ID is set (worker plane), resolves from the task's owner UDA.
// Otherwise (manager plane), resolves from the admin human's alias.
// Returns "system" as fallback if neither resolves.
func ResolveOwner() string {
	if jobID := os.Getenv("TTAL_JOB_ID"); jobID != "" {
		task, err := exportTaskByHexIDFn(jobID, "")
		if err != nil {
			return "system"
		}
		if task.Owner != "" {
			return task.Owner
		}
		return "system"
	}

	cfg, err := config.Load()
	if err != nil {
		return "system"
	}
	if cfg.AdminHuman != nil && cfg.AdminHuman.Alias != "" {
		return cfg.AdminHuman.Alias
	}
	return "system"
}
