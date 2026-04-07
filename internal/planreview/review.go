package planreview

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/temenos"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// buildPlanReviewerEnvParts constructs the environment variable list for a plan-reviewer session.
// TTAL_JOB_ID is set from task.HexID() so the reviewer can resolve the task context.
func buildPlanReviewerEnvParts(task *taskwarrior.Task, agentName string, rt runtime.Runtime) []string {
	return []string{
		fmt.Sprintf("%s=%s", temenos.EnvKeyAgentName, agentName),
		fmt.Sprintf("%s=%s", temenos.EnvKeyJobID, task.HexID()),
		fmt.Sprintf("%s=%s", temenos.EnvKeyRuntime, rt),
	}
}

// SpawnPlanReviewer creates a new tmux window configured as a plan reviewer.
// workDir is the caller's working directory (project path) — used as the reviewer's cwd.
func SpawnPlanReviewer(
	sessionName string, task *taskwarrior.Task, reviewerName string, cfg *config.Config, workDir string,
) error {
	reviewerRT := cfg.DefaultRuntime()

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	envParts := buildPlanReviewerEnvParts(task, reviewerName, reviewerRT)

	var shellCmd string
	var mcpPath string
	if reviewerRT == runtime.Codex {
		// Codex reviewers stay on the old task-file path until #321.
		// Build prompt from template for Codex since it doesn't support the context hook.
		systemPrompt := buildPlanReviewerPrompt(cfg, task.UUID, reviewerRT)
		if systemPrompt == "" {
			return fmt.Errorf("plan_review prompt not configured: add [prompts] plan_review = \"...\" to config.toml")
		}
		promptFile, err := writePromptFile(systemPrompt)
		if err != nil {
			return err
		}
		codexCmd, err := launchcmd.BuildCodexGatekeeperCommand(ttalBin, promptFile)
		if err != nil {
			return err
		}
		shellCmd = cfg.BuildEnvShellCommand(envParts, codexCmd)
	} else {
		// Claude Code: register temenos session for MCP bash, then launch CC.
		regCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		mcpPath = temenos.RegisterReviewerTemenos(
			regCtx, reviewerName, workDir,
			task.UUID, temenos.TokenAnnotationPlanReviewer, reviewerRT,
		)
		ccCmd := launchcmd.BuildCCDirectCommand(ttalBin, reviewerName, "Review the plan.", mcpPath)
		shellCmd = cfg.BuildEnvShellCommand(envParts, ccCmd)
	}

	if err := tmux.NewWindow(sessionName, reviewerName, workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create plan-review window: %w", err)
	}

	fmt.Printf("Plan reviewer spawned in '%s' window\n", reviewerName)
	return nil
}

// RequestReReview sends a re-review message to the existing plan-review window.
// designerComment is the triage body from the designer; if non-empty it is written
// to a temp file and its path is injected via {{designer-comment}}.
func RequestReReview(sessionName, reviewerName string, designerComment string, cfg *config.Config) error {
	var commentRef string
	if designerComment != "" {
		// Temp file is intentionally not deleted — the reviewer reads it at their
		// own pace and OS /tmp cleanup handles eventual removal (mirrors PR loop pattern).
		f, err := os.CreateTemp("", "ttal-designer-comment-*.md")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create designer comment temp file: %v\n", err)
		} else {
			_, writeErr := f.WriteString(designerComment)
			_ = f.Close()
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write designer comment temp file: %v\n", writeErr)
			} else {
				commentRef = fmt.Sprintf("\nDesigner's triage: %s", f.Name())
			}
		}
	}

	tmpl := cfg.Prompt("plan_re_review")
	if tmpl == "" {
		return tmux.SendKeys(sessionName, reviewerName,
			"Plan has been revised. Re-review and post findings via ttal comment add.")
	}

	replacer := strings.NewReplacer(
		"{{designer-comment}}", commentRef,
	)
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", cfg.DefaultRuntime())
	return tmux.SendKeys(sessionName, reviewerName, msg)
}

func buildPlanReviewerPrompt(cfg *config.Config, taskUUID string, rt runtime.Runtime) string {
	tmpl := cfg.Prompt("plan_review")
	if tmpl == "" {
		return ""
	}
	replacer := strings.NewReplacer("{{task-id}}", taskUUID)
	return config.RenderTemplate(replacer.Replace(tmpl), "", rt)
}

func writePromptFile(prompt string) (string, error) {
	f, err := os.CreateTemp("", "plan-review-prompt-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create plan review prompt file: %w", err)
	}
	if _, err := f.WriteString(prompt); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to write plan review prompt: %w", err)
	}
	_ = f.Close()
	return f.Name(), nil
}
