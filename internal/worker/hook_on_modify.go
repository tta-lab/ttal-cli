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

	// Task completion is handled by the daemon's completion poller (poll.go),
	// which owns the full lifecycle: check PR → close worker → mark done → notify.

	// No matching event — pass through
	passthroughTask(modified)
}
