package worker

import (
	"fmt"
	"os"
)

// HookOnModify is the main taskwarrior on-modify hook entry point.
func HookOnModify() {
	original, modified, rawModified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-modify: " + err.Error())
		if len(rawModified) > 0 {
			fmt.Println(string(rawModified))
		}
		os.Exit(0)
	}

	// Re-enrich when project changes to a non-empty value.
	if newProject := modified.Project(); newProject != "" && newProject != original.Project() {
		enrichInline(modified)
	}

	writeTask(modified)
}
