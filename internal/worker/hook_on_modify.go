package worker

import "os"

// HookOnModify is the main taskwarrior on-modify hook entry point.
func HookOnModify() {
	_, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-modify: " + err.Error())
		os.Exit(0)
	}

	// Pass through — on-start routing removed (use ttal task execute/design/research).
	passthroughTask(modified)
}
