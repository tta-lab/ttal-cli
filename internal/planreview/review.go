package planreview

import (
	"fmt"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var (
	tmuxNewWindowFn = tmux.NewWindow
	osExecFn        = os.Executable
)

// SpawnPlanReviewer creates a new tmux window configured as a plan reviewer.
// workDir is the caller's working directory (project path) — used as the reviewer's cwd.
// rt is the pre-resolved runtime for the reviewer agent (caller resolves via agentfs.ResolveRuntime).
// readOnly is the pre-resolved sandbox access flag (caller resolves via agentfs.ResolveAccess);
// when true and rt is Lenos, --readonly is forwarded to lenos for temenos sandbox enforcement.
func SpawnPlanReviewer(
	sessionName string, task *taskwarrior.Task, reviewerName string,
	rt runtime.Runtime, readOnly bool, cfg *config.Config, workDir string,
) error {
	ttalBin, err := osExecFn()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	envParts := launchcmd.BuildEnvParts(task.HexID(), reviewerName, rt)
	launchCmd, err := launchcmd.BuildAgentLaunchCommand(
		rt, ttalBin, reviewerName, readOnly, true, launchcmd.WakeTrigger(), "",
	)
	if err != nil {
		return fmt.Errorf("build plan-reviewer launch command: %w", err)
	}
	shellCmd := cfg.BuildEnvShellCommand(envParts, launchCmd)

	if err := tmuxNewWindowFn(sessionName, reviewerName, workDir, shellCmd); err != nil {
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
	msg := config.RenderTemplate(replacer.Replace(tmpl), "", runtime.Runtime(cfg.DefaultRuntime))
	return tmux.SendKeys(sessionName, reviewerName, msg)
}
