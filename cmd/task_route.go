package cmd

import (
	"fmt"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/worker"
)

// routeTaskToAgent sends a task assignment message to a named agent via the daemon.
func routeTaskToAgent(agentName, taskUUID, roleTag, rolePrompt string) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	uuid := task.UUID
	if len(uuid) > 8 {
		uuid = uuid[:8]
	}

	msg := fmt.Sprintf("[%s] %s — %s\n%s",
		roleTag, uuid, task.Description, rolePrompt)

	return daemon.Send(daemon.SendRequest{
		To:      agentName,
		Message: msg,
	})
}

// spawnWorkerForTask spawns a worker for a task using the standard spawn flow.
// When dryRun is true, it prints what would happen without spawning.
func spawnWorkerForTask(taskUUID string, dryRun bool) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	if task.ProjectPath == "" {
		return fmt.Errorf(
			"task %s has no project_path — run enrichment first "+
				"(task add usually triggers this automatically)",
			taskUUID)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	rt := cfg.WorkerRuntime()
	for _, t := range task.Tags {
		switch t {
		case string(runtime.OpenCode), "oc":
			rt = runtime.OpenCode
		case string(runtime.Codex), "cx":
			rt = runtime.Codex
		}
	}

	workerName := strings.TrimPrefix(task.Branch, "worker/")
	if workerName == "" {
		workerName = task.SessionName()
	}

	if dryRun {
		fmt.Printf("Task:        %s\n", task.Description)
		fmt.Printf("UUID:        %s\n", task.UUID)
		fmt.Printf("Project:     %s\n", task.ProjectPath)
		fmt.Printf("Runtime:     %s\n", rt)
		fmt.Printf("Worker:      %s\n", workerName)
		branch := task.Branch
		if branch == "" {
			branch = "(auto-generated)"
		}
		fmt.Printf("Branch:      %s\n", branch)
		fmt.Printf("Session:     %s\n", task.SessionName())
		return nil
	}

	if err := taskwarrior.StartTask(task.UUID); err != nil {
		// Ignore "already active" — task may be re-executed after a failed worker
		if !strings.Contains(err.Error(), "already active") {
			return fmt.Errorf("task start failed before worker spawn: %w", err)
		}
	}

	if err := worker.Spawn(worker.SpawnConfig{
		Name:     workerName,
		Project:  task.ProjectPath,
		TaskUUID: task.UUID,
		Worktree: true,
		Yolo:     true,
		Runtime:  rt,
	}); err != nil {
		return err
	}

	// Notify lifecycle agent — fire-and-forget
	worker.NotifyTelegram(fmt.Sprintf("🚀 Worker spawned: %s\nTask: %s", workerName, task.Description))

	return nil
}
