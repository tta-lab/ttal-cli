package daemon

import (
	"log"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// handleTaskComplete processes a taskComplete HTTP request and delivers
// task-done notifications to manager agents and optionally the spawner.
func handleTaskComplete(req TaskCompleteRequest, mcfg *config.DaemonConfig, registry *adapterRegistry) SendResponse {
	if req.Team == "" {
		req.Team = config.DefaultTeamName
	}

	// Build target — reuse prWatchTarget with only the fields notifications need.
	// Owner, Repo, Provider, SessionName are left zero — not used by notify functions.
	var prIndex int64
	if req.PRID != "" {
		if info, err := taskwarrior.ParsePRID(req.PRID); err == nil {
			prIndex = info.Index
		}
	}
	target := prWatchTarget{
		TaskUUID:    req.TaskUUID,
		Team:        req.Team,
		Spawner:     req.Spawner,
		Description: req.Desc,
		PRIndex:     prIndex,
	}

	notifyManagerAgents(mcfg, registry, target)
	if req.Spawner != "" {
		notifySpawnerMerged(mcfg, registry, target)
		log.Printf("[taskComplete] notified managers + spawner %q for task %s",
			req.Spawner, shortSHA(req.TaskUUID))
	} else {
		log.Printf("[taskComplete] notified managers for task %s", shortSHA(req.TaskUUID))
	}
	return SendResponse{OK: true}
}
