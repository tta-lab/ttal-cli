package worker

import "fmt"

// isCompletionEvent detects when a task transitions to completed status.
func isCompletionEvent(original, modified hookTask) bool {
	return original.Status() != taskStatusCompleted && modified.Status() == taskStatusCompleted
}

// formatCompletionMessage builds the notification text for a completed task.
func formatCompletionMessage(task hookTask) string {
	return fmt.Sprintf("[task completed] %s (uuid: %s)", task.Description(), task.UUID())
}

// handleOnComplete sends a notification to the task scheduler agent's tmux session.
// Fire-and-forget: logs errors but does not propagate them.
func handleOnComplete(modified hookTask) {
	defer passthroughTask(modified)

	agent := resolveTaskSchedulerAgent()
	if agent == "" {
		hookLog("COMPLETE_SKIP", modified.UUID(), modified.Description(),
			"reason", "no_task_scheduler_agent")
		return
	}

	msg := formatCompletionMessage(modified)

	// Send via daemon socket (To-only routing → handleTo → tmux send-keys)
	err := sendToDaemon(daemonSendRequest{To: agent, Message: msg})
	if err != nil {
		hookLogFile(fmt.Sprintf("ERROR: completion notify failed for %s: %v", agent, err))
		return
	}

	hookLog("COMPLETE_NOTIFY", modified.UUID(), modified.Description(),
		"agent", agent, "status", "sent")
}
