package planreview

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestSpawnPlanReviewer_ClaudeCodeBranch(t *testing.T) {
	origNewWindow := tmuxNewWindowFn
	origExec := osExecFn
	origWake := launchcmd.WakeTriggerForRuntimeFn
	defer func() {
		tmuxNewWindowFn = origNewWindow
		osExecFn = origExec
		launchcmd.WakeTriggerForRuntimeFn = origWake
	}()
	launchcmd.WakeTriggerForRuntimeFn = func(runtime.Runtime) string { return launchcmd.ContextTrigger }

	var capturedShellCmd string
	tmuxNewWindowFn = func(session, window, workDir, shellCmd string) error {
		capturedShellCmd = shellCmd
		return nil
	}

	osExecFn = func() (string, error) {
		return "/usr/bin/ttal", nil
	}

	task := &taskwarrior.Task{UUID: "abcd1234-0000-0000-0000-000000000000"}
	cfg := &config.Config{DefaultRuntime: "claude-code"}

	err := SpawnPlanReviewer("test-session", task, "plan-review-lead", runtime.ClaudeCode, false, cfg, "/tmp")
	if err != nil {
		t.Fatalf("SpawnPlanReviewer: %v", err)
	}

	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger:\n  got: %q\n  want to contain: %q",
			capturedShellCmd, launchcmd.ContextTrigger)
	}
	if !strings.Contains(capturedShellCmd, "claude --dangerously-skip-permissions --agent plan-review-lead") {
		t.Errorf("expected claude agent, got: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "lenos") {
		t.Errorf("should not contain lenos: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "Review the plan.") {
		t.Errorf("shellCmd still has old 'Review the plan.' trigger:\n  %q", capturedShellCmd)
	}
}

func TestSpawnPlanReviewer_LenosBranch(t *testing.T) {
	origNewWindow := tmuxNewWindowFn
	origExec := osExecFn
	origWake := launchcmd.WakeTriggerForRuntimeFn
	defer func() {
		tmuxNewWindowFn = origNewWindow
		osExecFn = origExec
		launchcmd.WakeTriggerForRuntimeFn = origWake
	}()
	launchcmd.WakeTriggerForRuntimeFn = func(runtime.Runtime) string { return launchcmd.ContextTrigger }

	var capturedShellCmd string
	tmuxNewWindowFn = func(session, window, workDir, shellCmd string) error {
		capturedShellCmd = shellCmd
		return nil
	}

	osExecFn = func() (string, error) {
		return "/usr/bin/ttal", nil
	}

	task := &taskwarrior.Task{UUID: "abcd1234-0000-0000-0000-000000000000"}
	cfg := &config.Config{DefaultRuntime: "lenos"}

	err := SpawnPlanReviewer("test-session", task, "plan-review-lead", runtime.Lenos, true, cfg, "/tmp")
	if err != nil {
		t.Fatalf("SpawnPlanReviewer: %v", err)
	}

	if !strings.Contains(capturedShellCmd, "lenos --agent plan-review-lead") {
		t.Errorf("expected lenos agent, got: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "--small-model") {
		t.Errorf("plan-reviewer lenos command should not use --small-model, got: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "claude") {
		t.Errorf("should not contain claude: %q", capturedShellCmd)
	}
	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger:\n  got: %q", capturedShellCmd)
	}
}
