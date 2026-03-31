package worker

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestBuildEnvParts(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}

	parts := buildEnvParts(task, runtime.ClaudeCode, "/custom/taskrc", "")

	if len(parts) < 2 {
		t.Fatal("expected at least TTAL_AGENT_NAME and TTAL_JOB_ID")
	}
	if parts[0] != "TTAL_AGENT_NAME=code-lead" {
		t.Errorf("first part should be TTAL_AGENT_NAME=code-lead, got %q", parts[0])
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

	parts := buildEnvParts(task, runtime.ClaudeCode, "", "")

	for _, p := range parts {
		if strings.HasPrefix(p, "TASKRC=") {
			t.Error("TASKRC should not be set when empty")
		}
	}
}

func TestWriteTaskFile_MissingCoderPrompt(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Runtime: runtime.Codex}
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}
	// Empty config has no coder prompt configured
	shellCfg := &config.Config{}
	_, err := writeTaskFile(task, cfg, shellCfg)
	if err == nil {
		t.Fatal("expected error when coder prompt not configured")
	}
	if !strings.Contains(err.Error(), "coder prompt not configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveRuntime(t *testing.T) {
	tests := []struct {
		name     string
		configRT runtime.Runtime
		taskTags []string
		want     runtime.Runtime
	}{
		{
			name:     "codex tag switches runtime when config empty",
			configRT: "",
			taskTags: []string{"codex"},
			want:     runtime.Codex,
		},
		{
			name:     "cx alias switches runtime",
			configRT: "",
			taskTags: []string{"cx"},
			want:     runtime.Codex,
		},
		{
			name:     "claude-code with codex tag switches to codex",
			configRT: runtime.ClaudeCode,
			taskTags: []string{"codex"},
			want:     runtime.Codex,
		},
		{
			name:     "empty config no tags defaults to claude-code",
			configRT: "",
			taskTags: nil,
			want:     runtime.ClaudeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &taskwarrior.Task{
				UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
				Description: "test task",
				Tags:        tt.taskTags,
			}
			got := resolveRuntime(tt.configRT, task)
			if got != tt.want {
				t.Errorf("resolveRuntime(%q, %v) = %q, want %q", tt.configRT, tt.taskTags, got, tt.want)
			}
		})
	}
}
