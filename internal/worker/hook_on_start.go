package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guion-opensource/ttal-cli/ent/agent"
	"github.com/guion-opensource/ttal-cli/ent/tag"
	"github.com/guion-opensource/ttal-cli/internal/db"
)

const defaultAgent = "worker-lifecycle"

// HookOnStart handles the task start (+ACTIVE) event.
// Reads two JSON lines from stdin, outputs modified task to stdout.
func HookOnStart() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-start: " + err.Error())
		os.Exit(0)
	}

	// Only handle transitions to +ACTIVE
	if original.Start() != "" || modified.Start() == "" || modified.Status() != "pending" {
		outputModifiedTask(modified)
		return
	}

	handleOnStart(original, modified)
}

// handleOnStart contains the start logic, callable from HookOnStart or HookOnModify.
func handleOnStart(_ hookTask, modified hookTask) {
	defer outputModifiedTask(modified)

	hookLog("START", modified.UUID(), modified.Description())

	taskTags := modified.Tags()
	if len(taskTags) == 0 {
		// No tags → default agent
		message := extractTaskContext(modified)
		notifyAgent(message)
		hookLog("AGENT_NOTIFY", modified.UUID(), modified.Description(),
			"reason", "task_started", "agent", defaultAgent)
		return
	}

	// Find matching agent by tag overlap
	matchedAgent := findMatchingAgent(taskTags)
	if matchedAgent == "" {
		// No match → default agent (kestrel/worker-lifecycle)
		message := extractTaskContext(modified)
		notifyAgent(message)
		hookLog("AGENT_NOTIFY", modified.UUID(), modified.Description(),
			"reason", "task_started", "agent", defaultAgent)
		return
	}

	// Matched an agent → route to it
	message := extractTaskContext(modified)
	notifyAgentWith(message, matchedAgent)
	hookLog("AGENT_NOTIFY", modified.UUID(), modified.Description(),
		"reason", "task_started", "agent", matchedAgent)
}

// findMatchingAgent queries the ttal database for an agent whose tags
// overlap with the given task tags. Returns the agent name or empty string.
func findMatchingAgent(taskTags []string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	dbPath := filepath.Join(home, ".ttal", "ttal.db")
	if _, err := os.Stat(dbPath); err != nil {
		hookLogFile("DB not found at " + dbPath + ", skipping agent routing")
		return ""
	}

	database, err := db.New(dbPath)
	if err != nil {
		hookLogFile("ERROR opening DB for agent routing: " + err.Error())
		return ""
	}
	defer database.Close()

	ctx := context.Background()

	// Find agents that have at least one tag matching the task's tags
	agents, err := database.Agent.Query().
		Where(agent.HasTagsWith(tag.NameIn(taskTags...))).
		WithTags().
		All(ctx)
	if err != nil {
		hookLogFile("ERROR querying agents: " + err.Error())
		return ""
	}

	if len(agents) == 0 {
		return ""
	}

	// Return the first matching agent
	// If multiple match, pick the one with most overlapping tags
	best := agents[0]
	bestOverlap := 0

	tagSet := make(map[string]bool, len(taskTags))
	for _, t := range taskTags {
		tagSet[strings.ToLower(t)] = true
	}

	for _, ag := range agents {
		overlap := 0
		for _, t := range ag.Edges.Tags {
			if tagSet[t.Name] {
				overlap++
			}
		}
		if overlap > bestOverlap {
			best = ag
			bestOverlap = overlap
		}
	}

	hookLog("ROUTE", best.Name, fmt.Sprintf("matched %d tags", bestOverlap),
		"task_tags", strings.Join(taskTags, ","))

	return best.Name
}
