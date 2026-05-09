package owner

import (
	"os"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// ExportTaskByHexIDFn allows test injection of the task lookup function.
var ExportTaskByHexIDFn = taskwarrior.ExportTaskByHexID

// FallbackOwner is returned when no owner can be resolved.
const FallbackOwner = "system"

// ResolveOwner resolves the owner for the current session.
//
// If TTAL_JOB_ID is set (worker plane), resolves from the task's owner UDA.
// Otherwise (manager plane), resolves from the admin human's alias.
// Returns FallbackOwner if neither resolves.
func ResolveOwner() string {
	if jobID := os.Getenv("TTAL_JOB_ID"); jobID != "" {
		task, err := ExportTaskByHexIDFn(jobID, "")
		if err != nil {
			return FallbackOwner
		}
		if task.Owner != "" {
			return task.Owner
		}
		return FallbackOwner
	}

	cfg, err := config.Load()
	if err != nil {
		return FallbackOwner
	}
	if cfg.AdminHuman != nil && cfg.AdminHuman.Alias != "" {
		return cfg.AdminHuman.Alias
	}
	return FallbackOwner
}
