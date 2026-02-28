package worker

import (
	"fmt"
	"os"
)

// HookOnAdd handles the taskwarrior on-add event.
// Reads one JSON line from stdin, enriches inline if project is set, outputs to stdout.
func HookOnAdd() {
	task, rawLine, err := readHookAddInput()
	if err != nil {
		hookLogFile("ERROR in on-add: " + err.Error())
		if len(rawLine) > 0 {
			fmt.Println(string(rawLine))
		}
		os.Exit(0)
	}

	hookLog("ADD", task.UUID(), task.Description())

	// Inline enrichment — no subprocess needed.
	if task.Project() != "" {
		enrichInline(task)
	}

	writeTask(task)
}
