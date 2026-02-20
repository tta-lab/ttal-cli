package worker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

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

	// Build context from description + annotations
	taskContext := task.Description
	for _, ann := range task.Annotations {
		taskContext += "\n" + ann.Description
	}

	prompt := buildEnrichPrompt(taskContext, task.UUID)

	ctx, cancel := context.WithTimeout(context.Background(), enrichTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", "--allowedTools", "Bash")
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR running claude for %s: %v\nOutput: %s", uuid, err, string(out)))
		notifyTelegram(fmt.Sprintf("⚠ Enrichment failed (claude): %s\n%v", task.Description, err))
		return
	}

	hookLogFile(fmt.Sprintf("enrich: completed for task %s\nOutput: %s", uuid, string(out)))
}

func buildEnrichPrompt(taskContext, uuid string) string {
	//nolint:lll // prompt template reads better as a single block
	return fmt.Sprintf(`You are a task enrichment agent. Your job is to enrich a taskwarrior task with project_path and branch UDAs so it can be automatically spawned as a worker.

TASK UUID: %s

TASK CONTEXT:
%s

INSTRUCTIONS:
1. Run: ttal project list
2. From the task context, identify which project this task belongs to
3. Run: ttal project get <alias> to get the project path
4. Derive a short, kebab-case branch name from the task description (e.g., "fix-auth-timeout", "add-user-api")
5. Run: task %s modify project_path:<path> branch:worker/<branch-name>
6. Print a one-line summary of what you set

RULES:
- Branch name should be descriptive but short (2-4 words, kebab-case)
- If you cannot determine the project, do NOT modify the task — just print "SKIP: could not determine project"
- Do not add any other modifications
- Do not start the task`, uuid, taskContext, uuid)
}
