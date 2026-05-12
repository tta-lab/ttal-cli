package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestSpawnReviewer_ClaudeCodeBranch(t *testing.T) {
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

	err := SpawnReviewer("test-session", ctx, "pr-review-lead", runtime.ClaudeCode, false, cfg, "/tmp")
	if err != nil {
		t.Fatalf("SpawnReviewer: %v", err)
	}

	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger:\n  got: %q\n  want to contain: %q",
			capturedShellCmd, launchcmd.ContextTrigger)
	}
	if !strings.Contains(capturedShellCmd, "claude --dangerously-skip-permissions --agent pr-review-lead") {
		t.Errorf("expected claude agent, got: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "lenos") {
		t.Errorf("should not contain lenos: %q", capturedShellCmd)
	}
}

func TestSpawnReviewer_LenosBranch(t *testing.T) {
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

	task := &taskwarrior.Task{
		UUID:    "abcd1234-0000-0000-0000-000000000000",
		PRID:    "42",
		Project: "test-project",
	}
	agentRoot := t.TempDir()
	agentDir := filepath.Join(agentRoot, "pr-review-lead")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\nname: pr-review-lead\nlenos:\n  pair_with: coder\n---\n# PR Review\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultRuntime: "lenos",
		Sync:           config.SyncConfig{WorkerAgentPaths: []string{agentRoot}},
	}
	ctx := &pr.Context{
		Task:  task,
		Owner: "test-owner",
		Repo:  "test-repo",
	}

	err := SpawnReviewer("test-session", ctx, "pr-review-lead", runtime.Lenos, true, cfg, "/tmp")
	if err != nil {
		t.Fatalf("SpawnReviewer: %v", err)
	}

	if !strings.Contains(capturedShellCmd, "lenos --agent pr-review-lead") {
		t.Errorf("expected lenos agent, got: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "--small-model") {
		t.Errorf("reviewer lenos command should not use --small-model, got: %q", capturedShellCmd)
	}
	if !strings.Contains(capturedShellCmd, "--pair-with 'coder'") {
		t.Errorf("expected PR reviewer pair target, got: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "claude") {
		t.Errorf("should not contain claude: %q", capturedShellCmd)
	}
	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger:\n  got: %q", capturedShellCmd)
	}
}
