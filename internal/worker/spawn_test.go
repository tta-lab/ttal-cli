package worker

import (
	"strings"
	"testing"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

func TestBuildEnvParts(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}

	parts := buildEnvParts(task, runtime.ClaudeCode, "/custom/taskrc")

	if len(parts) < 2 {
		t.Fatal("expected at least TTAL_ROLE and TTAL_JOB_ID")
	}
	if parts[0] != "TTAL_ROLE=coder" {
		t.Errorf("first part should be TTAL_ROLE=coder, got %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], "TTAL_JOB_ID=") {
		t.Errorf("second part should be TTAL_JOB_ID, got %q", parts[1])
	}

	// Check TTAL_RUNTIME is included
	foundRuntime := false
	for _, p := range parts {
		if p == "TTAL_RUNTIME=claude-code" {
			foundRuntime = true
		}
	}
	if !foundRuntime {
		t.Error("expected TTAL_RUNTIME=claude-code in env parts")
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

	parts := buildEnvParts(task, runtime.ClaudeCode, "")

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

	cfg := SpawnConfig{Name: "test", Yolo: true, Runtime: runtime.ClaudeCode}
	envParts := []string{"TTAL_JOB_ID=test-id"}
	shellCfg := &config.Config{}

	cmd := buildClaudeCodeCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", task, envParts, shellCfg)

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

	cfg := SpawnConfig{Name: "test", Runtime: runtime.ClaudeCode}
	shellCfg := &config.Config{}
	cmd := buildClaudeCodeCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", task, nil, shellCfg)

	if !strings.Contains(cmd, "--model sonnet") {
		t.Error("CC command should use sonnet model when task has +sonnet tag")
	}
}
