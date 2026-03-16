package daemon

import (
	"log"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// handleTaskComplete processes a taskComplete HTTP request and delivers
// task-done notifications to manager agents, optionally the spawner, and Telegram.
func handleTaskComplete(req TaskCompleteRequest, mcfg *config.DaemonConfig, registry *adapterRegistry) SendResponse {
	if req.Team == "" {
		req.Team = config.DefaultTeamName
	}

	// Build target — reuse prWatchTarget with only the fields notifications need.
	// Owner, Repo, Provider, SessionName are left zero — not used by notify functions.
	var prIndex int64
	if req.PRID != "" {
		if info, err := taskwarrior.ParsePRID(req.PRID); err != nil {
			log.Printf("[taskComplete] ParsePRID %q failed: %v — PR index will be 0", req.PRID, err)
		} else {
			prIndex = info.Index
		}
	}

	// Use PR title if available, fall back to task description.
	desc := req.Desc
	if req.PRTitle != "" {
		desc = req.PRTitle
	}

	target := prWatchTarget{
		TaskUUID:    req.TaskUUID,
		Team:        req.Team,
		Spawner:     req.Spawner,
		Description: desc,
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
	// Only notify Telegram if there was a PR — plain task completions are silent.
	if req.PRID != "" {
		notifyTelegramTaskDone(mcfg, target)
	}
	return SendResponse{OK: true}
}

// notifyTelegramTaskDone sends a task-done Telegram message to the team's notification channel.
func notifyTelegramTaskDone(mcfg *config.DaemonConfig, target prWatchTarget) {
	teamName := target.Team
	if teamName == "" {
		teamName = config.DefaultTeamName
	}
	teamCfg, ok := mcfg.Teams[teamName]
	if !ok {
		log.Printf("[taskComplete] notifyTelegram: no config for team %q — skipped", teamName)
		return
	}
	msg := formatTaskDoneMsg(target)
	if err := notify.SendWithConfig(teamCfg.NotificationToken, teamCfg.ChatID, msg); err != nil {
		log.Printf("[taskComplete] telegram notify failed: %v", err)
	}
}
