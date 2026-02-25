package worker

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/enrichment"
	"codeberg.org/clawteam/ttal-cli/internal/project"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

// HookEnrich runs background task enrichment: resolves project_path from DB,
// then generates a branch name deterministically. Called as a detached subprocess by the on-add hook.
func HookEnrich(uuid string) {
	hookLogFile(fmt.Sprintf("enrich: starting for task %s", uuid))

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR exporting task %s: %v", uuid, err))
		NotifyTelegram(fmt.Sprintf("⚠ Enrichment failed (export): %s\n%v", uuid, err))
		return
	}

	// Always resolve project_path from DB — branch generation doesn't handle this
	projectPath := project.ResolveProjectPath(task.Project)

	if projectPath == "" {
		msg := fmt.Sprintf("⚠ Enrichment: no project match for task %s\n"+
			"  project field: %q\n"+
			"  task: %s\n"+
			"  Fix: set correct project with `task %s modify project:<alias>`",
			task.SessionID(), task.Project, task.Description, task.SessionID())
		hookLogFile(msg)
		NotifyTelegram(msg)
		return
	}

	hookLogFile(fmt.Sprintf("enrich: resolved project_path=%s from project=%s", projectPath, task.Project))
	enrichBranchOnly(task, projectPath)
}

// enrichBranchOnly generates a branch name deterministically, then sets both UDAs.
func enrichBranchOnly(task *taskwarrior.Task, projectPath string) {
	branch := enrichment.GenerateBranch(task.Description)
	if branch == "" {
		msg := fmt.Sprintf("⚠ Enrichment: could not generate branch for %s: %q", task.SessionID(), task.Description)
		hookLogFile(msg)
		NotifyTelegram(msg)
		return
	}

	branchWithPrefix := "worker/" + branch
	if err := taskwarrior.UpdateWorkerMetadata(task.UUID, branchWithPrefix, projectPath); err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR setting UDAs for %s: %v", task.UUID, err))
		NotifyTelegram(fmt.Sprintf("⚠ Enrichment failed (modify): %s\n%v", task.Description, err))
		return
	}

	hookLogFile(fmt.Sprintf("enrich: set project_path=%s branch=%s for %s", projectPath, branchWithPrefix, task.UUID))
}
