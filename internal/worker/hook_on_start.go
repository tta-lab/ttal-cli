package worker

import (
	"fmt"
	"os"
	"strings"
)

// HookOnStart handles the task start (+ACTIVE) event.
// Reads two JSON lines from stdin, outputs modified task to stdout.
func HookOnStart() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-start: " + err.Error())
		os.Exit(0)
	}

	if original.Start() != "" || modified.Start() == "" || modified.Status() != taskStatusPending {
		passthroughTask(modified)
		return
	}

	handleOnStart(original, modified)
}

// handleOnStart forks a background spawn if the task has enriched UDAs.
func handleOnStart(_ hookTask, modified hookTask) {
	defer passthroughTask(modified)

	hookLog("START", modified.UUID(), modified.Description())

	projectPath := modified.ProjectPath()
	branch := modified.Branch()

	if projectPath == "" || branch == "" {
		hookLog("START_SKIP", modified.UUID(), modified.Description(),
			"reason", "missing_udas", "project_path", projectPath, "branch", branch)
		notifyTelegram(fmt.Sprintf("⚠ Task started but missing UDAs (not enriched?):\n%s\nproject_path=%s branch=%s",
			modified.Description(), projectPath, branch))
		return
	}

	// Derive worker name from branch (worker/fix-auth → fix-auth)
	workerName := strings.TrimPrefix(branch, "worker/")

	// Fork background spawn
	if err := forkBackground("worker", "hook", "spawn-worker",
		modified.UUID(), workerName, projectPath); err != nil {
		hookLogFile(fmt.Sprintf("ERROR forking spawn for %s: %v", modified.UUID(), err))
		notifyTelegram(fmt.Sprintf("⚠ Failed to fork worker spawn:\n%s\nError: %v",
			modified.Description(), err))
		return
	}

	hookLog("START_SPAWN", modified.UUID(), modified.Description(),
		"worker", workerName, "project", projectPath, "status", "forked")
}
