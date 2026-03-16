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

	// Inline enrichment — validates project alias and generates branch.
	if task.Project() != "" {
		if err := enrichInline(task); err != nil {
			hookLogFile("ERROR: " + err.Error())
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	writeTask(task)
}
