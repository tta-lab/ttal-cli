package worker

import "os"

// HookOnModify is the main taskwarrior on-modify hook entry point.
func HookOnModify() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-modify: " + err.Error())
		os.Exit(0)
	}

	// Detect: Task Start (pending, no start → pending, has start)
	if original.Start() == "" && modified.Start() != "" && modified.Status() == taskStatusPending {
		handleOnStart(original, modified)
		return
	}

	// Detect: Task Complete (pending → completed)
	if original.Status() == taskStatusPending && modified.Status() == taskStatusCompleted {
		handleOnComplete(original, modified)
		return
	}

	// No matching event — pass through
	passthroughTask(modified)
}
