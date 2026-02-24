package worker

import (
	"fmt"
	"os"
)

// HookOnAdd handles the taskwarrior on-add event.
// Reads one JSON line from stdin, outputs it back to stdout.
// If the task's tags don't match any agent, forks background enrichment.
func HookOnAdd() {
	task, rawLine, err := readHookAddInput()
	if err != nil {
		hookLogFile("ERROR in on-add: " + err.Error())
		// Echo raw bytes so taskwarrior doesn't silently drop the task
		if len(rawLine) > 0 {
			fmt.Println(string(rawLine))
		}
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
	return resolveAgentNameByTags(taskTags) != ""
}
