package planreview

import (
	"fmt"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

const windowName = "plan-review"

// SpawnPlanReviewer creates a new tmux window configured as a plan reviewer.
func SpawnPlanReviewer(sessionName string, taskUUID string, cfg *config.Config) error {
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

	envParts := []string{"TTAL_AGENT_NAME=plan-reviewer", fmt.Sprintf("TTAL_RUNTIME=%s", reviewerRT)}
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
			}, model, "plan-review-lead", "Review the plan.",
		)
		if err != nil {
			return err
		}
		ccSessionPath = sessionPath
		shellCmd = cfg.BuildEnvShellCommand(envParts, resumeCmd)
	}

	if err := tmux.NewWindow(sessionName, windowName, workDir, shellCmd); err != nil {
		if ccSessionPath != "" {
			os.Remove(ccSessionPath)
		}
		return fmt.Errorf("failed to create plan-review window: %w", err)
	}

	fmt.Printf("Plan reviewer spawned in '%s' window\n", windowName)
	return nil
}

// RequestReReview sends a re-review message to the existing plan-review window.
func RequestReReview(sessionName string, taskUUID string, cfg *config.Config) error {
	tmpl := cfg.Prompt("plan_re_review")
	if tmpl == "" {
		return tmux.SendKeys(sessionName, windowName,
			fmt.Sprintf("Plan has been revised. Re-review task %s and post findings via ttal comment add.", taskUUID))
	}

	replacer := strings.NewReplacer(
		"{{task-id}}", taskUUID,
		"{{previous-findings}}", "",
	)
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", cfg.ReviewerRuntime())
	return tmux.SendKeys(sessionName, windowName, msg)
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
