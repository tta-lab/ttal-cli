package worker

import (
	"strings"
	"testing"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

func TestBuildEnvParts(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}

	parts := buildEnvParts(task, "/custom/taskrc")

	if len(parts) < 1 {
		t.Fatal("expected at least TTAL_JOB_ID")
	}
	if !strings.HasPrefix(parts[0], "TTAL_JOB_ID=") {
		t.Errorf("first part should be TTAL_JOB_ID, got %q", parts[0])
	}

	// Check taskrc is included
	found := false
	for _, p := range parts {
		if p == "TASKRC=/custom/taskrc" {
			found = true
		}
	}
	if !found {
		t.Error("expected TASKRC=/custom/taskrc in env parts")
	}
}

func TestBuildEnvParts_NoTaskRC(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}

	parts := buildEnvParts(task, "")

	for _, p := range parts {
		if strings.HasPrefix(p, "TASKRC=") {
			t.Error("TASKRC should not be set when empty")
		}
	}
}

func TestBuildClaudeCodeCmd(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
		Tags:        []string{},
	}

	cfg := SpawnConfig{Name: "test", Yolo: true, Runtime: RuntimeClaudeCode}
	envParts := []string{"TTAL_JOB_ID=test-id"}

	cmd := buildClaudeCodeCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", task, envParts)

	if !strings.Contains(cmd, "claude") {
		t.Error("CC command should contain 'claude'")
	}
	if !strings.Contains(cmd, "--model opus") {
		t.Error("CC command should default to opus model")
	}
	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Error("CC command should include yolo flag when Yolo=true")
	}
	if !strings.Contains(cmd, "gatekeeper") {
		t.Error("CC command should use gatekeeper wrapper")
	}
}

func TestBuildClaudeCodeCmd_Sonnet(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
		Tags:        []string{"sonnet"},
	}

	cfg := SpawnConfig{Name: "test", Runtime: RuntimeClaudeCode}
	cmd := buildClaudeCodeCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", task, nil)

	if !strings.Contains(cmd, "--model sonnet") {
		t.Error("CC command should use sonnet model when task has +sonnet tag")
	}
}

func TestBuildOpenCodeCmd(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Yolo: false, Runtime: RuntimeOpenCode}
	envParts := []string{"TTAL_JOB_ID=test-id"}

	cmd := buildOpenCodeCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", envParts)

	if !strings.Contains(cmd, "opencode run") {
		t.Error("OC command should contain 'opencode run'")
	}
	if !strings.Contains(cmd, "--file") {
		t.Error("OC command should pass task file via --file")
	}
	if !strings.Contains(cmd, "gatekeeper") {
		t.Error("OC command should use gatekeeper wrapper")
	}
	if strings.Contains(cmd, "OPENCODE_PERMISSION") {
		t.Error("OC command should not contain OPENCODE_PERMISSION (set via tmux.SetEnv)")
	}
}

func TestBuildOpenCodeCmd_NoYoloInCommand(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Yolo: true, Runtime: RuntimeOpenCode}
	envParts := []string{"TTAL_JOB_ID=test-id"}

	cmd := buildOpenCodeCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", envParts)

	// OPENCODE_PERMISSION should NOT be in the fish command — it's set via tmux.SetEnv
	if strings.Contains(cmd, "OPENCODE_PERMISSION") {
		t.Error("OPENCODE_PERMISSION should be set via tmux.SetEnv, not in command string")
	}
}
