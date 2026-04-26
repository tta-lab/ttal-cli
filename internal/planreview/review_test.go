package planreview

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestBuildPlanReviewerEnvParts_ContainsJobID(t *testing.T) {
	task := &taskwarrior.Task{UUID: "f9a917aa-fc67-4aab-b398-18480e58ce86"}
	parts := buildPlanReviewerEnvParts(task, "plan-review-lead", runtime.ClaudeCode)
	var found bool
	for _, p := range parts {
		if p == "TTAL_JOB_ID=f9a917aa" {
			found = true
		}
	}
	if !found {
		t.Errorf("TTAL_JOB_ID=f9a917aa not found in env parts: %v", parts)
	}
}

func TestBuildPlanReviewerEnvParts_AgentNamePassthrough(t *testing.T) {
	task := &taskwarrior.Task{UUID: "abcd1234-0000-0000-0000-000000000000"}
	parts := buildPlanReviewerEnvParts(task, "custom-reviewer", runtime.ClaudeCode)
	var found bool
	for _, p := range parts {
		if p == "TTAL_AGENT_NAME=custom-reviewer" {
			found = true
		}
	}
	if !found {
		t.Errorf("TTAL_AGENT_NAME=custom-reviewer not found in env parts: %v", parts)
	}
}

func TestSpawnPlanReviewer_TriggerUsesContextTrigger(t *testing.T) {
	origNewWindow := tmuxNewWindowFn
	origExec := osExecFn
	defer func() {
		tmuxNewWindowFn = origNewWindow
		osExecFn = origExec
	}()

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

	err := SpawnPlanReviewer("test-session", task, "plan-review-lead", cfg, "/tmp")
	if err != nil {
		t.Fatalf("SpawnPlanReviewer: %v", err)
	}

	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger:\n  got: %q\n  want to contain: %q",
			capturedShellCmd, launchcmd.ContextTrigger)
	}
	if strings.Contains(capturedShellCmd, "Review the plan.") {
		t.Errorf("shellCmd still has old 'Review the plan.' trigger:\n  %q", capturedShellCmd)
	}
}
