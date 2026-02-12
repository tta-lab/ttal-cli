package worker

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// HookOnComplete handles the task completion event.
// Reads two JSON lines from stdin, outputs modified task to stdout.
func HookOnComplete() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-complete: " + err.Error())
		os.Exit(0)
	}

	// Only handle transitions to completed
	if original.Status() != "pending" || modified.Status() != "completed" {
		outputModifiedTask(modified)
		return
	}

	handleOnComplete(original, modified)
}

// handleOnComplete contains the completion logic, callable from HookOnComplete or HookOnModify.
func handleOnComplete(_ hookTask, modified hookTask) {
	defer outputModifiedTask(modified)

	sessionName := modified.SessionName()
	if sessionName == "" {
		hookLog("COMPLETE", modified.UUID(), modified.Description(), "worker", "none")
		return
	}

	start := time.Now()

	result, closeErr := Close(sessionName, false)
	duration := fmt.Sprintf("%ds", int(time.Since(start).Seconds()))

	if closeErr == nil && result != nil && result.Cleaned {
		// Auto-cleaned successfully
		hookLog("COMPLETE", modified.UUID(), modified.Description(),
			"worker", sessionName, "clean", "yes", "duration", duration)
		return
	}

	if errors.Is(closeErr, ErrNeedsDecision) && result != nil {
		// Needs manual decision — notify agent
		hookLog("COMPLETE", modified.UUID(), modified.Description(),
			"worker", sessionName, "merged", result.Merged,
			"clean", fmt.Sprintf("%t", result.Clean), "duration", duration)

		message := fmt.Sprintf(`Task %s (%s) completed, needs cleanup decision:

Worker: %s
Status: %s
State Dump: %s

Please decide whether to:
1. Keep worker (work still in progress)
2. Force cleanup with: ttal worker close %s --force`,
			modified.UUID(), modified.Description(),
			sessionName, result.Status, result.StateDump, sessionName)

		notifyAgent(message)
		hookLog("AGENT_NOTIFY", modified.UUID(), modified.Description(),
			"worker", sessionName, "reason", "needs_decision")
		return
	}

	// Error case — notify agent
	status := "Unknown error"
	if result != nil {
		status = result.Status
	} else if closeErr != nil {
		status = closeErr.Error()
	}

	hookLog("ERROR", modified.UUID(), modified.Description(),
		"worker", sessionName, "error", "cleanup_error")

	message := fmt.Sprintf(`Task %s (%s) completed but cleanup error:

Worker: %s
Error: %s

Check ~/.task/hooks.log for details.`,
		modified.UUID(), modified.Description(),
		sessionName, status)

	notifyAgent(message)
	hookLog("AGENT_NOTIFY", modified.UUID(), modified.Description(),
		"worker", sessionName, "reason", "cleanup_error")
}
