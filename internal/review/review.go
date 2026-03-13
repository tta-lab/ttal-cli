package review

import (
	"fmt"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

const windowName = "review"

// SpawnReviewer creates a new tmux window configured as a PR reviewer.
func SpawnReviewer(sessionName string, ctx *pr.Context, cfg *config.Config) error {
	if ctx.Task.PRID == "" {
		return fmt.Errorf("no PR associated with this task — run `ttal pr create` first")
	}

	prInfo, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}
	prIndex := prInfo.Index

	reviewerRT := cfg.ReviewerRuntime()

	prompt := buildReviewerPrompt(cfg, ctx, prIndex, reviewerRT)
	if prompt == "" {
		return fmt.Errorf("review prompt not configured: add [prompts] review = \"...\" to config.toml")
	}

	promptFile, err := writePromptFile(prompt)
	if err != nil {
		return err
	}

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	reviewerCmd, err := buildReviewerRuntimeCmd(ttalBin, promptFile, reviewerRT, cfg.ReviewerModel())
	if err != nil {
		return err
	}

	envParts := []string{"TTAL_ROLE=reviewer", fmt.Sprintf("TTAL_RUNTIME=%s", reviewerRT)}
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
// coderComment, if non-empty, is best-effort written to a temp file.
// If write succeeds, its path is included in the message so the reviewer
// can read the coder's triage update.
func RequestReReview(sessionName string, full bool, coderComment string, cfg *config.Config) error {
	var commentRef string
	if coderComment != "" {
		f, err := os.CreateTemp("", "ttal-coder-comment-*.md")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create coder comment temp file: %v\n", err)
		} else {
			_, writeErr := f.WriteString(coderComment)
			_ = f.Close()
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write coder comment temp file: %v\n", writeErr)
			} else {
				commentRef = fmt.Sprintf(" Coder's comment at %s —", f.Name())
			}
		}
	}

	scope := "scoped to new changes"
	if full {
		scope = "on all changes"
	}

	tmpl := cfg.Prompt("re_review")
	if tmpl == "" {
		return fmt.Errorf("re_review prompt not configured: add [prompts] re_review = \"...\" to config.toml")
	}
	replacer := strings.NewReplacer(
		"{{coder-comment}}", commentRef,
		"{{review-scope}}", scope,
	)
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", cfg.ReviewerRuntime())

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

func buildReviewerRuntimeCmd(ttalBin, promptFile string, rt runtime.Runtime, model string) (string, error) {
	return launchcmd.BuildGatekeeperCommand(ttalBin, promptFile, rt, model)
}

func writePromptFile(prompt string) (string, error) {
	f, err := os.CreateTemp("", "review-prompt-*.txt")
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
