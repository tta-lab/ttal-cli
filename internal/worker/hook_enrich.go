package worker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

const enrichTimeout = 60 * time.Second

// HookEnrich runs background task enrichment via claude -p --model haiku.
// Called as a detached subprocess by the on-add hook.
func HookEnrich(uuid string) {
	hookLogFile(fmt.Sprintf("enrich: starting for task %s", uuid))

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR exporting task %s: %v", uuid, err))
		notifyTelegram(fmt.Sprintf("⚠ Enrichment failed (export): %s\n%v", uuid, err))
		return
	}

	prompt := buildEnrichPrompt(task)

	ctx, cancel := context.WithTimeout(context.Background(), enrichTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", "--allowedTools", "Bash", "Read")
	// Run in data dir so CC writes JSONL there, not in the agent's watched directory.
	cmd.Dir = config.ResolveDataDir()
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR running claude for %s: %v\nOutput: %s", uuid, err, string(out)))
		notifyTelegram(fmt.Sprintf("⚠ Enrichment failed (claude): %s\n%v", task.Description, err))
		return
	}

	hookLogFile(fmt.Sprintf("enrich: completed for task %s\nOutput: %s", uuid, string(out)))
}

func buildEnrichPrompt(task *taskwarrior.Task) string {
	// Use FormatPrompt() which handles annotations, file refs, and inlines docs
	taskContext := task.FormatPrompt()

	// Add tags and project if present
	var metadata []string
	if task.Project != "" {
		metadata = append(metadata, fmt.Sprintf("Project: %s", task.Project))
	}
	if len(task.Tags) > 0 {
		metadata = append(metadata, fmt.Sprintf("Tags: %s", strings.Join(task.Tags, ", ")))
	}
	metadataSection := ""
	if len(metadata) > 0 {
		metadataSection = "\n" + strings.Join(metadata, "\n")
	}

	//nolint:lll // raw string prompt template
	return fmt.Sprintf(`You are a task enrichment agent. Your job is to enrich a taskwarrior task with project_path and branch UDAs so it can be automatically spawned as a worker.

TASK UUID: %s
%s
TASK CONTEXT:
%s
INSTRUCTIONS:
1. Run: ttal project list — read the descriptions to understand each project
2. Read any referenced documentation included above to understand the actual target project
3. Match the task to the correct project based on content, NOT based on where the plan file is stored
4. Run: ttal project get <alias> to get the project path
5. Derive a short, kebab-case branch name from the task description (e.g., "fix-auth-timeout", "add-user-api")
6. Run: task %s modify project_path:<path> branch:worker/<branch-name>
7. Print a one-line summary of what you set

RULES:
- Branch name should be descriptive but short (2-4 words, kebab-case)
- If you cannot determine the project, do NOT modify the task — just print "SKIP: could not determine project"
- Do not add any other modifications
- Do not start the task`, task.UUID, metadataSection, taskContext, task.UUID)
}
