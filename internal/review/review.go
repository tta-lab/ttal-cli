package review

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	envpkg "github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/temenos"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// buildReviewerEnvParts constructs the environment variable list for a PR reviewer session.
// TTAL_JOB_ID is set so the reviewer can resolve the task context via ttal pipeline prompt.
func buildReviewerEnvParts(task *taskwarrior.Task, agentName string, rt runtime.Runtime) []string {
	parts := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		fmt.Sprintf("TTAL_JOB_ID=%s", task.HexID()),
		fmt.Sprintf("TTAL_RUNTIME=%s", rt),
	}
	return parts
}

// buildReviewerSessionEnv returns the temenos session env map for a PR reviewer.
// Includes agent identity, task ID, runtime, and allowlisted .env vars.
func buildReviewerSessionEnv(task *taskwarrior.Task, agentName string, rt runtime.Runtime) map[string]string {
	m := map[string]string{
		"TTAL_AGENT_NAME": agentName,
		"TTAL_JOB_ID":     task.HexID(),
		"TTAL_RUNTIME":    string(rt),
	}
	if dotEnv := envpkg.AllowedDotEnvMap(); dotEnv != nil {
		for k, v := range dotEnv {
			m[k] = v
		}
	}
	return m
}

// SpawnReviewer creates a new tmux window configured as a PR reviewer.
// workDir is the caller's working directory (project path) — used as the reviewer's cwd.
func SpawnReviewer(sessionName string, ctx *pr.Context, reviewerName string, cfg *config.Config, workDir string) error {
	if ctx.Task.PRID == "" {
		return fmt.Errorf("no PR associated with this task — run `ttal pr create` first")
	}

	// Compute branch at runtime — works in both worktree and non-worktree setups.
	gitBranch := worker.CurrentBranch(ctx.Task.UUID, ctx.Task.Project, workDir)

	prInfo, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}
	prIndex := prInfo.Index

	reviewerRT := cfg.DefaultRuntime()

	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	var shellCmd string
	var mcpPath string

	envParts := buildReviewerEnvParts(ctx.Task, reviewerName, reviewerRT)

	if reviewerRT == runtime.Codex {
		// Codex reviewers stay on the old task-file path until #321.
		// Build prompt from template for Codex since it doesn't support the context hook.
		systemPrompt := buildReviewerPrompt(cfg, ctx, prIndex, reviewerRT, gitBranch)
		if systemPrompt == "" {
			return fmt.Errorf("review prompt not configured: add [prompts] review = \"...\" to config.toml")
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
		mcpPath = registerReviewerTemenos(ctx.Task, reviewerName, workDir, reviewerRT)
		ccCmd := launchcmd.BuildCCDirectCommand(ttalBin, reviewerName, "Review the PR.", mcpPath)
		shellCmd = cfg.BuildEnvShellCommand(envParts, ccCmd)
	}

	if err := tmux.NewWindow(sessionName, reviewerName, workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create reviewer window: %w", err)
	}

	fmt.Printf("Reviewer spawned in '%s' window\n", reviewerName)
	fmt.Printf("  Reviewing PR #%d in %s/%s\n", prIndex, ctx.Owner, ctx.Repo)
	return nil
}

// registerReviewerTemenos registers a temenos session for the PR reviewer and writes the MCP config.
// Best-effort: warns on failure and returns empty string so reviewer still launches.
func registerReviewerTemenos(task *taskwarrior.Task, reviewerName, workDir string, rt runtime.Runtime) string {
	ctx := context.Background()
	env := buildReviewerSessionEnv(task, reviewerName, rt)
	mcpJSON, token, err := temenos.RegisterSessionForAgent(ctx, reviewerName, []string{workDir}, "", env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register temenos session for reviewer (non-fatal): %v\n", err)
		return ""
	}
	// Annotate task with the reviewer token for cleanup on close.
	if annErr := taskwarrior.AnnotateTask(task.UUID, "temenos_pr_reviewer_token:"+token); annErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to annotate task with reviewer temenos token: %v\n", annErr)
	}
	mcpName := temenos.ReviewerMCPName(task.HexID(), "pr")
	path, err := temenos.WriteMCPConfigFile(mcpName, mcpJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write reviewer MCP config (non-fatal): %v\n", err)
		return ""
	}
	return path
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
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", cfg.DefaultRuntime())

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
