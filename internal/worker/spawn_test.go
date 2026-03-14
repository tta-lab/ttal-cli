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

func TestBuildLaunchCmd(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Runtime: runtime.ClaudeCode}
	envParts := []string{"TTAL_JOB_ID=test-id"}
	shellCfg := &config.Config{}

	cmd, err := buildLaunchCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", envParts, shellCfg, "sonnet")
	if err != nil {
		t.Fatalf("buildLaunchCmd returned error: %v", err)
	}

	if !strings.Contains(cmd, "claude") {
		t.Error("CC command should contain 'claude'")
	}
	if !strings.Contains(cmd, "--model sonnet") {
		t.Error("CC command should use sonnet model by default")
	}
	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Error("CC command should include yolo flag")
	}
	if !strings.Contains(cmd, "gatekeeper") {
		t.Error("CC command should use gatekeeper wrapper")
	}
}

func TestBuildLaunchCmd_OpusModel(t *testing.T) {
	cfg := SpawnConfig{Name: "test", Runtime: runtime.ClaudeCode}
	shellCfg := &config.Config{}

	cmd, err := buildLaunchCmd(cfg, "/usr/bin/ttal", "/tmp/task.txt", nil, shellCfg, "opus")
	if err != nil {
		t.Fatalf("buildLaunchCmd returned error: %v", err)
	}
	if !strings.Contains(cmd, "--model opus") {
		t.Errorf("CC command should use opus model, got: %s", cmd)
	}
}

func TestBuildLaunchCmd_RejectsUnsupportedRuntime(t *testing.T) {
	_, err := buildLaunchCmd(
		SpawnConfig{Runtime: "unknown"},
		"/usr/bin/ttal",
		"/tmp/task.txt",
		nil,
		&config.Config{},
		"sonnet",
	)
	if err == nil {
		t.Fatal("expected error for unsupported worker runtime")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported runtime error, got: %v", err)
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
