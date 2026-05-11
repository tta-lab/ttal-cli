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

var (
	tmuxNewWindowFn = tmux.NewWindow
	osExecFn        = os.Executable
)

// SpawnReviewer creates a new tmux window configured as a PR reviewer.
// workDir is the caller's working directory (project path) — used as the reviewer's cwd.
// rt is the pre-resolved runtime for the reviewer agent (caller resolves via agentfs.ResolveRuntime).
// readOnly is the pre-resolved sandbox access flag (caller resolves via agentfs.ResolveAccess);
// when true and rt is Lenos, --readonly is forwarded to lenos for temenos sandbox enforcement.
func SpawnReviewer(
	sessionName string, ctx *pr.Context, reviewerName string,
	rt runtime.Runtime, readOnly bool, cfg *config.Config, workDir string,
) error {
	if ctx.Task.PRID == "" {
		return fmt.Errorf("no PR associated with this task — run `ttal pr create` first")
	}

	prInfo, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}

	ttalBin, err := osExecFn()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	envParts := launchcmd.BuildEnvParts(ctx.Task.HexID(), reviewerName, rt)
	launchCmd, err := launchcmd.BuildAgentLaunchCommand(
		rt, ttalBin, reviewerName, readOnly, false, launchcmd.WakeTriggerForRuntime(rt), "",
	)
	if err != nil {
		return fmt.Errorf("build reviewer launch command: %w", err)
	}
	shellCmd := cfg.BuildEnvShellCommand(envParts, launchCmd)

	if err := tmuxNewWindowFn(sessionName, reviewerName, workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create reviewer window: %w", err)
	}

	fmt.Printf("Reviewer spawned in '%s' window\n", reviewerName)
	fmt.Printf("  Reviewing PR #%d in %s/%s\n", prInfo.Index, ctx.Owner, ctx.Repo)
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
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", runtime.Runtime(cfg.DefaultRuntime))

	fmt.Println("Sending re-review request to existing reviewer window...")
	return tmux.SendKeys(sessionName, reviewerName, msg)
}

// ResolveSessionName returns the name of the current tmux session.
func ResolveSessionName() (string, error) {
	return tmux.CurrentSession()
}
