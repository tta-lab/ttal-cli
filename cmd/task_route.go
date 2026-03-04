package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	gitutil "github.com/tta-lab/ttal-cli/internal/git"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
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
		printDryRun(task, rt, workerName)
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

	return nil
}

func printDryRun(task *taskwarrior.Task, rt runtime.Runtime, workerName string) {
	fmt.Printf("Task:        %s\n", task.Description)
	fmt.Printf("UUID:        %s\n", task.UUID)
	fmt.Printf("Project:     %s\n", task.ProjectPath)

	if gitRoot, err := gitutil.FindRoot(task.ProjectPath); err == nil {
		resolvedProject, _ := filepath.EvalSymlinks(task.ProjectPath)
		resolvedRoot, _ := filepath.EvalSymlinks(gitRoot)
		if resolvedProject != resolvedRoot {
			if rel, err := filepath.Rel(gitRoot, task.ProjectPath); err == nil {
				fmt.Printf("Git root:    %s\n", gitRoot)
				fmt.Printf("Subpath:     %s\n", rel)
			}
		}
	}

	fmt.Printf("Runtime:     %s\n", rt)
	fmt.Printf("Worker:      %s\n", workerName)
	branch := task.Branch
	if branch == "" {
		branch = "(auto-generated)"
	}
	fmt.Printf("Branch:      %s\n", branch)
	fmt.Printf("Session:     %s\n", task.SessionName())
}
