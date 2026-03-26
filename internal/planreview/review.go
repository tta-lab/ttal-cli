package planreview

import (
	"fmt"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// buildPlanReviewerEnvParts constructs the environment variable list for a plan-reviewer session.
// TTAL_JOB_ID is set so the reviewer can resolve the task context via ExportTaskBySessionID.
func buildPlanReviewerEnvParts(taskUUID string, agentName string, rt runtime.Runtime) ([]string, error) {
	if len(taskUUID) < 8 {
		return nil, fmt.Errorf("taskUUID too short to derive job ID: %q", taskUUID)
	}
	readOnlyPaths := env.CollectReadOnlyPaths()
	temenosEnv, err := env.ReviewerTemenosEnv(readOnlyPaths)
	if err != nil {
		return nil, fmt.Errorf("build temenos env for plan reviewer: %w", err)
	}
	parts := make([]string, 0, 3+len(temenosEnv))
	parts = append(parts,
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		fmt.Sprintf("TTAL_JOB_ID=%s", taskUUID[:8]),
		fmt.Sprintf("TTAL_RUNTIME=%s", rt),
	)
	// Temenos MCP sandbox config — plan reviewers get read-only cwd access plus project paths
	parts = append(parts, temenosEnv...)
	return parts, nil
}

// SpawnPlanReviewer creates a new tmux window configured as a plan reviewer.
func SpawnPlanReviewer(sessionName string, taskUUID string, reviewerName string, cfg *config.Config) error {
	reviewerRT := cfg.ReviewerRuntime()
	model := cfg.ReviewerModel()

	systemPrompt := buildPlanReviewerPrompt(cfg, taskUUID, reviewerRT)
	if systemPrompt == "" {
		return fmt.Errorf("plan_review prompt not configured: add [prompts] plan_review = \"...\" to config.toml")
	}

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	envParts, err := buildPlanReviewerEnvParts(taskUUID, reviewerName, reviewerRT)
	if err != nil {
		return err
	}
	var ccSessionPath string

	var shellCmd string
	if reviewerRT == runtime.Codex {
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
		sessionPath, resumeCmd, err := launchcmd.BuildCCSessionCommand(
			ttalBin, workDir, breathe.SessionConfig{
				CWD:     workDir,
				Handoff: systemPrompt,
			}, model, reviewerName, "Review the plan.",
		)
		if err != nil {
			return err
		}
		ccSessionPath = sessionPath
		shellCmd = cfg.BuildEnvShellCommand(envParts, resumeCmd)
	}

	if err := tmux.NewWindow(sessionName, reviewerName, workDir, shellCmd); err != nil {
		if ccSessionPath != "" {
			os.Remove(ccSessionPath)
		}
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
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", cfg.ReviewerRuntime())
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
