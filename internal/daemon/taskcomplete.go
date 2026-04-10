package daemon

import (
	"context"
	"log"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// handleTaskComplete processes a taskComplete HTTP request and delivers
// task-done notifications to manager agents, optionally the owner, and frontend.
func handleTaskComplete(
	req TaskCompleteRequest, cfg *config.Config,
	registry *adapterRegistry, frontends map[string]frontend.Frontend,
) SendResponse {
	// Use PR title if available, fall back to task description.
	desc := req.Desc
	if req.PRTitle != "" {
		desc = req.PRTitle
	}

	var prIndex int64
	if req.PRID != "" {
		if info, err := taskwarrior.ParsePRID(req.PRID); err != nil {
			log.Printf("[taskComplete] ParsePRID %q failed: %v — PR index will be 0", req.PRID, err)
		} else {
			prIndex = info.Index
		}
	}

	target := prWatchTarget{
		TaskUUID:    req.TaskUUID,
		Owner:       req.Owner,
		Description: desc,
		PRIndex:     prIndex,
	}

	notifyManagerAgents(cfg, registry, frontends, target)
	if req.Owner != "" {
		notifyOwnerMerged(cfg, registry, frontends, target)
		log.Printf("[taskComplete] notified managers + owner %q for task %s",
			req.Owner, shortSHA(req.TaskUUID))
	} else {
		log.Printf("[taskComplete] notified managers for task %s", shortSHA(req.TaskUUID))
	}
	// Only notify Telegram if there was a PR — plain task completions are silent.
	if req.PRID != "" {
		notifyTelegramTaskDone(frontends, target)
	}
	return SendResponse{OK: true}
}

// notifyTelegramTaskDone sends a task-done notification to the team's frontend.
func notifyTelegramTaskDone(frontends map[string]frontend.Frontend, target prWatchTarget) {
	teamName := target.Team
	if teamName == "" {
		teamName = "default"
	}
	fe, ok := frontends[teamName]
	if !ok {
		log.Printf("[taskComplete] notifyTelegram: no frontend for team %q — skipped", teamName)
		return
	}
	if err := fe.SendNotification(context.Background(), formatTaskDoneMsg(target)); err != nil {
		log.Printf("[taskComplete] notify failed: %v", err)
	}
}
