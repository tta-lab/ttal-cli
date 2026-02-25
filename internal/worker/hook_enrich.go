package worker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/project"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

const enrichTimeout = 60 * time.Second

// HookEnrich runs background task enrichment: resolves project_path from DB,
// then asks haiku only for a branch name. Called as a detached subprocess by the on-add hook.
func HookEnrich(uuid string) {
	hookLogFile(fmt.Sprintf("enrich: starting for task %s", uuid))

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR exporting task %s: %v", uuid, err))
		NotifyTelegram(fmt.Sprintf("⚠ Enrichment failed (export): %s\n%v", uuid, err))
		return
	}

	// Always resolve project_path from DB — haiku never picks the project
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

// enrichBranchOnly asks haiku for just a branch name (no tools), then sets both UDAs.
func enrichBranchOnly(task *taskwarrior.Task, projectPath string) {
	prompt := buildBranchPrompt(task)

	ctx, cancel := context.WithTimeout(context.Background(), enrichTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku")
	cmd.Dir = config.ResolveDataDir()
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR branch generation for %s: %v\nOutput: %s", task.UUID, err, string(out)))
		NotifyTelegram(fmt.Sprintf("⚠ Enrichment failed (branch): %s\n%v", task.Description, err))
		return
	}

	branch := parseBranchFromOutput(string(out))
	if branch == "" {
		msg := fmt.Sprintf("⚠ Enrichment: could not parse branch name for %s\nHaiku output: %s",
			task.SessionID(), string(out))
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

func buildBranchPrompt(task *taskwarrior.Task) string {
	return fmt.Sprintf(`Generate a git branch name for this task. Output ONLY the branch name, nothing else.

Rules:
- Short, kebab-case, 2-4 words (e.g., "fix-auth-timeout", "add-voice-config")
- Descriptive of the task content
- No "worker/" prefix (that's added automatically)

Task: %s
Description: %s`, task.UUID, task.Description)
}

func parseBranchFromOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ToLower(line)
		line = strings.ReplaceAll(line, " ", "-")
		// Strip anything that's not a-z, 0-9, or hyphen
		var clean strings.Builder
		for _, r := range line {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				clean.WriteRune(r)
			}
		}
		result := strings.Trim(clean.String(), "-")
		if result != "" {
			return result
		}
	}
	return ""
}
