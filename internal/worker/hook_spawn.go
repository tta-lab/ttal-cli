package worker

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

// HookSpawnWorker runs worker spawn as a background process and reports to Telegram.
// Called as a detached subprocess by the on-start hook.
func HookSpawnWorker(uuid, workerName, projectPath string) {
	hookLogFile(fmt.Sprintf("spawn-worker: starting %s for task %s in %s", workerName, uuid, projectPath))

	// Load task to get description for notifications
	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		hookLogFile(fmt.Sprintf("spawn-worker: ERROR exporting task %s: %v", uuid, err))
		notifyTelegram(fmt.Sprintf("⚠ Worker spawn failed: %s\nError: could not load task: %v", workerName, err))
		return
	}

	err = Spawn(SpawnConfig{
		Name:     workerName,
		Project:  projectPath,
		TaskUUID: uuid,
		Worktree: true,
		Yolo:     true,
	})

	if err != nil {
		hookLogFile(fmt.Sprintf("spawn-worker: ERROR spawning %s: %v", workerName, err))
		notifyTelegram(fmt.Sprintf("⚠ Worker spawn failed: %s\nTask: %s\nError: %v",
			workerName, task.Description, err))
		return
	}

	hookLogFile(fmt.Sprintf("spawn-worker: successfully spawned %s", workerName))
	notifyTelegram(fmt.Sprintf("🚀 Worker spawned: %s\nTask: %s\nProject: %s",
		workerName, task.Description, projectPath))
}
