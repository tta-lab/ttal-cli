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

	parts := buildEnvParts(task, runtime.ClaudeCode, CoderAgentName, nil)

	if len(parts) < 2 {
		t.Fatal("expected at least TTAL_AGENT_NAME and TTAL_JOB_ID")
	}
	if parts[0] != "TTAL_AGENT_NAME=coder" {
		t.Errorf("first part should be TTAL_AGENT_NAME=coder, got %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], "TTAL_JOB_ID=") {
		t.Errorf("second part should be TTAL_JOB_ID, got %q", parts[1])
	}

	foundRuntime := false
	for _, p := range parts {
		if p == "TTAL_RUNTIME=claude-code" {
			foundRuntime = true
		}
	}
	if !foundRuntime {
		t.Error("expected TTAL_RUNTIME=claude-code in env parts")
	}
}

func TestBuildEnvParts_BasicEnvParts(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}

	parts := buildEnvParts(task, runtime.ClaudeCode, CoderAgentName, nil)

	for _, p := range parts {
		if strings.HasPrefix(p, "TASKRC=") {
			t.Error("TASKRC should not be set in env parts")
		}
	}
}


func TestWriteTaskFile_MissingCoderPrompt(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Runtime: runtime.Codex}
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}
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
		want     runtime.Runtime
	}{
		{
			name:     "explicit claude-code",
			configRT: runtime.ClaudeCode,
			want:     runtime.ClaudeCode,
		},
		{
			name:     "explicit codex",
			configRT: runtime.Codex,
			want:     runtime.Codex,
		},
		{
			name:     "explicit lenos",
			configRT: runtime.Lenos,
			want:     runtime.Lenos,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRuntime(tt.configRT, nil)
			if got != tt.want {
				t.Errorf("resolveRuntime(%q) = %q, want %q", tt.configRT, got, tt.want)
			}
		})
	}
}
