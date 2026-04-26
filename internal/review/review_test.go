package review

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestBuildReviewerEnvParts_AgentName(t *testing.T) {
	task := &taskwarrior.Task{UUID: "abc12345-0000-0000-0000-000000000000"}
	parts := buildReviewerEnvParts(task, "pr-review-lead", runtime.ClaudeCode)
	var foundAgent, foundJobID bool
	for _, p := range parts {
		if p == "TTAL_AGENT_NAME=pr-review-lead" {
			foundAgent = true
		}
		if p == "TTAL_JOB_ID=abc12345" {
			foundJobID = true
		}
	}
	if !foundAgent {
		t.Errorf("TTAL_AGENT_NAME=pr-review-lead not found in env parts: %v", parts)
	}
	if !foundJobID {
		t.Errorf("TTAL_JOB_ID=abc12345 not found in env parts: %v", parts)
	}
}

func TestSpawnReviewer_TriggerUsesContextTrigger(t *testing.T) {
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

	task := &taskwarrior.Task{
		UUID:    "abcd1234-0000-0000-0000-000000000000",
		PRID:    "42",
		Project: "test-project",
	}
	cfg := &config.Config{DefaultRuntime: "claude-code"}
	ctx := &pr.Context{
		Task:  task,
		Owner: "test-owner",
		Repo:  "test-repo",
	}

	err := SpawnReviewer("test-session", ctx, "pr-review-lead", cfg, "/tmp")
	if err != nil {
		t.Fatalf("SpawnReviewer: %v", err)
	}

	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger:\n  got: %q\n  want to contain: %q",
			capturedShellCmd, launchcmd.ContextTrigger)
	}
}
