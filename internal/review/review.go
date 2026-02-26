package review

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/pr"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

const windowName = "review"

// SpawnReviewer creates a new tmux window with a Claude Code instance
// configured as a PR reviewer.
func SpawnReviewer(sessionName string, ctx *pr.Context) error {
	if ctx.Task.PRID == "" {
		return fmt.Errorf("no PR associated with this task — run `ttal pr create` first")
	}

	prIndex, err := strconv.ParseInt(ctx.Task.PRID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}

	prompt := buildReviewerPrompt(ctx, prIndex)

	promptFile, err := writePromptFile(prompt)
	if err != nil {
		return err
	}

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	claudeCmd := fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- claude --model opus --dangerously-skip-permissions --",
		ttalBin, promptFile)

	shellCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	envParts := []string{"TTAL_ROLE=reviewer"}
	if rt := os.Getenv("TTAL_RUNTIME"); rt != "" {
		envParts = append(envParts, "TTAL_RUNTIME="+rt)
	}
	shellCmd := shellCfg.BuildEnvShellCommand(envParts, claudeCmd)

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
func RequestReReview(sessionName string, full bool, coderComment string) error {
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
	msg := fmt.Sprintf(
		"Worker has pushed fixes addressing your review.%s Please re-review:"+
			" 1. Run /pr-review %s"+
			" 2. Post updated review via: ttal pr comment create \"your review\" (NEVER use --no-review)"+
			" 3. End with VERDICT: LGTM if all issues addressed, or VERDICT: NEEDS_WORK if not",
		commentRef, scope)

	fmt.Println("Sending re-review request to existing reviewer window...")
	return tmux.SendKeys(sessionName, windowName, msg)
}

func buildReviewerPrompt(ctx *pr.Context, prIndex int64) string {
	return fmt.Sprintf(`You are a code reviewer for PR #%d — "%s" in %s/%s.
Branch: %s

## Your Task

1. Run /pr-review to perform a comprehensive code review
   - Review scope: ONLY changes in this PR (the diff), not the entire codebase
   - Focus on: correctness, security, architecture, tests

2. Structure your findings as a PR comment with clear sections:
   - Critical Issues (must fix before merge)
   - Important Issues (should fix)
   - Suggestions (nice to have)
   - Strengths (what's well done)

3. Post your review using:
   ttal pr comment create "your structured review"

4. End your comment with one of:
   - VERDICT: NEEDS_WORK (if any critical issues)
   - VERDICT: LGTM (if no critical issues)

Do NOT merge the PR. The coder handles merging after triage.

## Important
- Only review what changed in the PR, not pre-existing code
- Be specific: reference file:line for each finding
- Be constructive: suggest fixes, not just problems
- If you're unsure about something, say so rather than raising a false alarm
- NEVER use --no-review flag when posting comments — your review must trigger the coder to triage
`,
		prIndex, ctx.Task.Description, ctx.Owner, ctx.Repo, ctx.Task.Branch)
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
