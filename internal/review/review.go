package review

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// buildReviewerEnvParts constructs the environment variable list for a PR reviewer session.
func buildReviewerEnvParts(agentName string, rt runtime.Runtime) []string {
	readOnlyPaths := env.CollectReadOnlyPaths()
	temenosEnv := env.ReviewerTemenosEnv(readOnlyPaths)
	parts := make([]string, 0, 2+len(temenosEnv))
	parts = append(parts,
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		fmt.Sprintf("TTAL_RUNTIME=%s", rt),
	)
	// Temenos MCP sandbox config — reviewers get read-only cwd access plus project paths
	parts = append(parts, temenosEnv...)
	return parts
}

// SpawnReviewer creates a new tmux window configured as a PR reviewer.
func SpawnReviewer(sessionName string, ctx *pr.Context, reviewerName string, cfg *config.Config) error {
	if ctx.Task.PRID == "" {
		return fmt.Errorf("no PR associated with this task — run `ttal pr create` first")
	}

	// Compute branch at runtime from the worktree — soft failure, review can proceed with empty branch.
	gitBranch, err := worker.WorktreeBranch(ctx.Task.UUID, ctx.Task.Project)
	if err != nil {
		shortUUID := ctx.Task.UUID
		if len(shortUUID) > 8 {
			shortUUID = shortUUID[:8]
		}
		log.Printf("[review] warning: could not resolve worktree branch for %s: %v", shortUUID, err)
	}

	prInfo, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}
	prIndex := prInfo.Index

	reviewerRT := cfg.ReviewerRuntime()
	model := cfg.ReviewerModel()

	systemPrompt := buildReviewerPrompt(cfg, ctx, prIndex, reviewerRT, gitBranch)
	if systemPrompt == "" {
		return fmt.Errorf("review prompt not configured: add [prompts] review = \"...\" to config.toml")
	}

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	var shellCmd string

	envParts := buildReviewerEnvParts(reviewerName, reviewerRT)
	var ccSessionPath string // non-empty for CC reviewers; cleaned up if tmux.NewWindow fails

	if reviewerRT == runtime.Codex {
		// Codex reviewers stay on the old task-file path until #321
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
		// Claude Code: JSONL session + trigger
		sessionPath, resumeCmd, err := launchcmd.BuildCCSessionCommand(
			ttalBin, workDir, breathe.SessionConfig{
				CWD:       workDir,
				GitBranch: gitBranch,
				Handoff:   systemPrompt,
			}, model, reviewerName, "Review the PR.",
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
		return fmt.Errorf("failed to create reviewer window: %w", err)
	}

	fmt.Printf("Reviewer spawned in '%s' window\n", reviewerName)
	fmt.Printf("  Reviewing PR #%d in %s/%s\n", prIndex, ctx.Owner, ctx.Repo)
	return nil
}

// RequestReReview sends a re-review message to the existing reviewer window.
// If full is true, requests a full re-review of all PR changes.
// If full is false, requests a delta re-review of only new changes.
// coderComment, if non-empty, is best-effort written to a temp file.
// If write succeeds, its path is included in the message so the reviewer
// can read the coder's triage update.
func RequestReReview(sessionName, reviewerName string, full bool, coderComment string, cfg *config.Config) error {
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
	return tmux.SendKeys(sessionName, reviewerName, msg)
}

func buildReviewerPrompt(cfg *config.Config, ctx *pr.Context, prIndex int64, rt runtime.Runtime, branch string) string {
	tmpl := cfg.Prompt("review")
	replacer := strings.NewReplacer(
		"{{pr-number}}", fmt.Sprintf("%d", prIndex),
		"{{pr-title}}", ctx.Task.Description,
		"{{owner}}", ctx.Owner,
		"{{repo}}", ctx.Repo,
		"{{branch}}", branch,
	)
	return config.RenderTemplate(replacer.Replace(tmpl), "", rt)
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
