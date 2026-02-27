package worker

import (
	"fmt"
	"os"
)

// HookOnAdd handles the taskwarrior on-add event.
// Reads one JSON line from stdin, outputs it back to stdout.
// Forks background enrichment for every new task.
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

	// Fork background enrichment
	if err := forkBackground("worker", "hook", "enrich", task.UUID()); err != nil {
		hookLogFile("ERROR forking enrichment: " + err.Error())
		return
	}

	hookLog("ADD_ENRICH", task.UUID(), task.Description(), "status", "forked")
}
