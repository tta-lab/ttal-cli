package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/ent/tag"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/db"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

// handleOnStart routes a started task to either an existing agent session (tag match)
// or spawns a new worker (UDA match).
//
// Resolution order:
//  1. Tag-based: match task tags against registered agents → send to existing session via daemon
//  2. UDA-based: use project_path and branch UDAs set by enrichment → spawn new worker
func handleOnStart(_ hookTask, modified hookTask) {
	defer passthroughTask(modified)

	hookLog("START", modified.UUID(), modified.Description())

	// 1. Try tag-based routing to existing agent session
	if agentName := resolveAgentNameByTags(modified.Tags()); agentName != "" {
		hookLog("START_ROUTE", modified.UUID(), modified.Description(),
			"method", "tag_match", "agent", agentName)
		routeToAgent(agentName, modified)
		return
	}

	// 2. Fall back to UDA-based worker spawn
	spawnFromUDAs(modified)
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

// spawnFromUDAs spawns a new worker using project_path and branch UDAs.
func spawnFromUDAs(modified hookTask) {
	projectPath := modified.ProjectPath()
	branch := modified.Branch()

	if projectPath == "" || branch == "" {
		hookLog("START_SKIP", modified.UUID(), modified.Description(),
			"reason", "missing_udas", "project_path", projectPath, "branch", branch)
		NotifyTelegram(fmt.Sprintf("⚠ Task started but missing UDAs (not enriched?):\n%s\nproject_path=%s branch=%s",
			modified.Description(), projectPath, branch))
		return
	}

	workerName := strings.TrimPrefix(branch, "worker/")

	// Detect runtime from task tags (+opencode/+oc or +codex/+cx)
	rt := runtime.ClaudeCode
	for _, t := range modified.Tags() {
		switch t {
		case string(runtime.OpenCode), "oc":
			rt = runtime.OpenCode
		case string(runtime.Codex), "cx":
			rt = runtime.Codex
		}
	}

	if err := forkBackground("worker", "hook", "spawn-worker",
		"--runtime", string(rt),
		modified.UUID(), workerName, projectPath); err != nil {
		hookLogFile(fmt.Sprintf("ERROR forking spawn for %s: %v", modified.UUID(), err))
		NotifyTelegram(fmt.Sprintf("⚠ Failed to fork worker spawn:\n%s\nError: %v",
			modified.Description(), err))
		return
	}

	hookLog("START_SPAWN", modified.UUID(), modified.Description(),
		"worker", workerName, "project", projectPath, "runtime", string(rt), "status", "forked")
}

// resolveAgentNameByTags queries the DB for an agent whose tags overlap with the task tags.
// Returns the agent's name, or empty string if no match.
func resolveAgentNameByTags(taskTags []string) string {
	if len(taskTags) == 0 {
		return ""
	}

	dbPath := filepath.Join(config.ResolveDataDir(), "ttal.db")
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
