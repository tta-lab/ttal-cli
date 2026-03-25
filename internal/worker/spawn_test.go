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

	parts, err := buildEnvParts(task, runtime.ClaudeCode, "/custom/taskrc")
	if err != nil {
		t.Fatalf("buildEnvParts() error: %v", err)
	}

	if len(parts) < 2 {
		t.Fatal("expected at least TTAL_AGENT_NAME and TTAL_JOB_ID")
	}
	if parts[0] != "TTAL_AGENT_NAME=coder" {
		t.Errorf("first part should be TTAL_AGENT_NAME=coder, got %q", parts[0])
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

	parts, err := buildEnvParts(task, runtime.ClaudeCode, "")
	if err != nil {
		t.Fatalf("buildEnvParts() error: %v", err)
	}

	for _, p := range parts {
		if strings.HasPrefix(p, "TASKRC=") {
			t.Error("TASKRC should not be set when empty")
		}
	}
}

func TestWriteSessionPrompt_MissingExecutePrompt(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Runtime: runtime.ClaudeCode}
	task := &taskwarrior.Task{
		UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
		Description: "test task",
	}
	// Empty config has no execute prompt configured
	shellCfg := &config.Config{}
	_, err := writeSessionPrompt(task, cfg, shellCfg)
	if err == nil {
		t.Fatal("expected error when execute prompt not configured")
	}
	if !strings.Contains(err.Error(), "execute prompt not configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "default returns sonnet",
			tags: nil,
			want: "sonnet",
		},
		{
			name: "hard tag returns opus",
			tags: []string{"hard"},
			want: "opus",
		},
		{
			name: "other tags return sonnet",
			tags: []string{"urgent", "frontend"},
			want: "sonnet",
		},
		{
			name: "hard with other tags still returns opus",
			tags: []string{"urgent", "hard", "frontend"},
			want: "opus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &taskwarrior.Task{
				UUID:        "abcdef01-2345-6789-abcd-ef0123456789",
				Description: "test task",
				Tags:        tt.tags,
			}
			// WorkerModel() accessor with configured values is tested in config_test.go.
			// Here we test the +hard override logic with default config.
			shellCfg := &config.Config{}
			got := resolveModel(task, shellCfg)
			if got != tt.want {
				t.Errorf("resolveModel(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
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
