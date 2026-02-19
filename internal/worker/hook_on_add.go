package worker

import (
	"context"
	"os"
	"path/filepath"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/ent/tag"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

// HookOnAdd handles the taskwarrior on-add event.
// Reads one JSON line from stdin, outputs it back to stdout.
// If the task's tags don't match any agent, forks background enrichment.
func HookOnAdd() {
	task, err := readHookAddInput()
	if err != nil {
		hookLogFile("ERROR in on-add: " + err.Error())
		os.Exit(0)
	}
	defer passthroughTask(task)

	hookLog("ADD", task.UUID(), task.Description())

	// Skip enrichment if task tags already match an agent
	if tagsMatchAgent(task.Tags()) {
		hookLog("ADD_SKIP", task.UUID(), task.Description(), "reason", "tags_match_agent")
		return
	}

	// Fork background enrichment
	if err := forkBackground("worker", "hook", "enrich", task.UUID()); err != nil {
		hookLogFile("ERROR forking enrichment: " + err.Error())
		return
	}

	hookLog("ADD_ENRICH", task.UUID(), task.Description(), "status", "forked")
}

// tagsMatchAgent checks if any of the given tags match a registered agent's tags.
func tagsMatchAgent(taskTags []string) bool {
	if len(taskTags) == 0 {
		return false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	dbPath := filepath.Join(home, ".ttal", "ttal.db")
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}

	database, err := db.New(dbPath)
	if err != nil {
		return false
	}
	defer database.Close() //nolint:errcheck

	count, err := database.Agent.Query().
		Where(agent.HasTagsWith(tag.NameIn(taskTags...))).
		Count(context.Background())
	if err != nil {
		return false
	}

	return count > 0
}
