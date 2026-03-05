package review

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

const windowName = "review"

// SpawnReviewer creates a new tmux window with a Claude Code instance
// configured as a PR reviewer.
func SpawnReviewer(sessionName string, ctx *pr.Context, cfg *config.Config, rt runtime.Runtime) error {
	if ctx.Task.PRID == "" {
		return fmt.Errorf("no PR associated with this task — run `ttal pr create` first")
	}

	prIndex, err := strconv.ParseInt(ctx.Task.PRID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}

	prompt := buildReviewerPrompt(cfg, ctx, prIndex, rt)

	promptFile, err := writePromptFile(prompt)
	if err != nil {
		return err
	}

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	reviewerCmd := buildReviewerRuntimeCmd(ttalBin, promptFile, rt)

	envParts := []string{"TTAL_ROLE=reviewer"}
	if rtEnv := os.Getenv("TTAL_RUNTIME"); rtEnv != "" {
		envParts = append(envParts, "TTAL_RUNTIME="+rtEnv)
	}
	shellCmd := cfg.BuildEnvShellCommand(envParts, reviewerCmd)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if err := tmux.NewWindow(sessionName, windowName, workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create reviewer window: %w", err)
	}

	fmt.Println("Reviewer spawned in 'review' window")
	fmt.Printf("  Reviewing PR #%d in %s/%s\n", prIndex, ctx.Owner, ctx.Repo)
	return nil
}

// RequestReReview sends a re-review message to the existing reviewer window.
// If full is true, requests a full re-review of all PR changes.
// If full is false, requests a delta re-review of only new changes.
// coderComment, if non-empty, is written to a temp file and its path included
// in the message so the reviewer can read the coder's triage update.
func RequestReReview(sessionName string, full bool, coderComment string, cfg *config.Config, rt runtime.Runtime) error {
	var commentRef string
	if coderComment != "" {
		f, err := os.CreateTemp("", "ttal-coder-comment-*.md")
		if err == nil {
			_, writeErr := f.WriteString(coderComment)
			_ = f.Close()
			if writeErr == nil {
				commentRef = fmt.Sprintf(" Coder's comment at %s —", f.Name())
			}
		}
	}

	scope := "scoped to new changes"
	if full {
		scope = "on all changes"
	}

	tmpl := cfg.Prompt("re_review")
	replacer := strings.NewReplacer(
		"{{coder-comment}}", commentRef,
		"{{review-scope}}", scope,
	)
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", rt)

	fmt.Println("Sending re-review request to existing reviewer window...")
	return tmux.SendKeys(sessionName, windowName, msg)
}

func buildReviewerPrompt(cfg *config.Config, ctx *pr.Context, prIndex int64, rt runtime.Runtime) string {
	tmpl := cfg.Prompt("review")
	replacer := strings.NewReplacer(
		"{{pr-number}}", fmt.Sprintf("%d", prIndex),
		"{{pr-title}}", ctx.Task.Description,
		"{{owner}}", ctx.Owner,
		"{{repo}}", ctx.Repo,
		"{{branch}}", ctx.Task.Branch,
	)
	return config.RenderTemplate(replacer.Replace(tmpl), "", rt)
}

// buildReviewerRuntimeCmd returns the runtime-specific reviewer launch command.
// Reviewers always run in permissive mode to avoid interactive permission stalls.
func buildReviewerRuntimeCmd(ttalBin, promptFile string, rt runtime.Runtime) string {
	switch rt {
	case runtime.OpenCode:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- opencode --prompt",
			ttalBin, promptFile)
	case runtime.Codex:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- codex --yolo --prompt",
			ttalBin, promptFile)
	default:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- claude --model opus --dangerously-skip-permissions --",
			ttalBin, promptFile)
	}
}

func writePromptFile(prompt string) (string, error) {
	f, err := os.CreateTemp("", "claude-review-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create review prompt file: %w", err)
	}
	if _, err := f.WriteString(prompt); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to write review prompt: %w", err)
	}
	_ = f.Close()
	return f.Name(), nil
}

// ResolveSessionName returns the name of the current tmux session.
func ResolveSessionName() (string, error) {
	return tmux.CurrentSession()
}
