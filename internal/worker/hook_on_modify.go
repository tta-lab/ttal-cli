package worker

import "os"

// HookOnModify is the main taskwarrior on-modify hook entry point.
// Reads two JSON lines from stdin, detects event type, and dispatches
// to the appropriate handler. Always outputs modified task JSON to stdout
// and exits 0 to avoid blocking taskwarrior.
func HookOnModify() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-modify: " + err.Error())
		os.Exit(0)
	}

	// Detect: Task Start (pending, no start → pending, has start)
	if original.Start() == "" && modified.Start() != "" && modified.Status() == "pending" {
		handleOnStart(original, modified)
		return
	}

	// Detect: Task Complete (pending → completed)
	if original.Status() == "pending" && modified.Status() == "completed" {
		handleOnComplete(original, modified)
		return
	}

	// No matching event — pass through
	outputModifiedTask(modified)
}
