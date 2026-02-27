package worker

import (
	"context"
	"fmt"
	"os"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/ent/tag"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

// handleOnStart routes a started task to an existing agent session via tag matching.
// Worker spawning is now handled by `ttal task execute`.
func handleOnStart(_ hookTask, modified hookTask) {
	defer passthroughTask(modified)

	hookLog("START", modified.UUID(), modified.Description())

	// Tag-based routing only — e.g., +newskill/+newagent → agent with matching tags
	if agentName := resolveAgentNameByTags(modified.Tags()); agentName != "" {
		hookLog("START_ROUTE", modified.UUID(), modified.Description(),
			"method", "tag_match", "agent", agentName)
		routeToAgent(agentName, modified)
		return
	}

	hookLog("START_SKIP", modified.UUID(), modified.Description(),
		"reason", "no_tag_match")
}

// routeToAgent sends the task to an existing agent session via the daemon.
// Sends a short message with the task ID — the agent uses `ttal task get` for full details.
func routeToAgent(agentName string, task hookTask) {
	uuid := task.UUID()
	if len(uuid) > 8 {
		uuid = uuid[:8]
	}
	msg := fmt.Sprintf("[task assigned] %s\nUUID: %s\nRun `ttal task get %s` for full details.",
		task.Description(), uuid, uuid)

	err := sendToDaemon(daemonSendRequest{
		To:      agentName,
		Message: msg,
	})
	if err != nil {
		hookLogFile(fmt.Sprintf("ERROR routing task to %s: %v", agentName, err))
		NotifyTelegram(fmt.Sprintf("⚠ Failed to route task to %s:\n%s\nError: %v",
			agentName, task.Description(), err))
		return
	}

	hookLog("START_ROUTED", task.UUID(), task.Description(),
		"agent", agentName, "status", "delivered")
}

// resolveAgentNameByTags queries the DB for an agent whose tags overlap with the task tags.
// Returns the agent's name, or empty string if no match.
func resolveAgentNameByTags(taskTags []string) string {
	if len(taskTags) == 0 {
		return ""
	}

	dbPath := config.ResolveDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}

	database, err := db.New(dbPath)
	if err != nil {
		hookLogFile(fmt.Sprintf("resolveAgentNameByTags: failed to open DB: %v", err))
		return ""
	}
	defer database.Close()

	matched, err := database.Agent.Query().
		Where(agent.HasTagsWith(tag.NameIn(taskTags...))).
		First(context.Background())
	if err != nil {
		return ""
	}

	return matched.Name
}
