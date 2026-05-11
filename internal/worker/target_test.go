package worker

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestTmuxTarget_Validate(t *testing.T) {
	tests := []struct {
		name    string
		target  TmuxTarget
		wantErr string
	}{
		{
			name:    "valid target",
			target:  TmuxTarget{Session: "ttal-default-astra", Window: "coder", WorkDir: "/tmp/wt"},
			wantErr: "",
		},
		{
			name:    "empty session",
			target:  TmuxTarget{Session: "", Window: "coder", WorkDir: "/tmp/wt"},
			wantErr: "session is empty",
		},
		{
			name:    "empty window",
			target:  TmuxTarget{Session: "ttal-default-astra", Window: "", WorkDir: "/tmp/wt"},
			wantErr: "window is empty",
		},
		{
			name:    "window with colon",
			target:  TmuxTarget{Session: "ttal-default-astra", Window: "coder:pr", WorkDir: "/tmp/wt"},
			wantErr: "contains ':'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !containsStr(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestResolveTmuxTarget_NilTask(t *testing.T) {
	_, err := ResolveTmuxTarget(nil)
	if err == nil {
		t.Fatal("expected error for nil task")
	}
	if !containsStr(err.Error(), "task is nil") {
		t.Errorf("expected 'task is nil', got %q", err.Error())
	}
}

func TestResolveTmuxTarget_MissingOwner(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abc12345-0000-0000-0000-000000000000",
		Description: "test task",
		Project:     "ttal",
	}
	_, err := ResolveTmuxTarget(task)
	if err == nil {
		t.Fatal("expected error for missing owner")
	}
	if !containsStr(err.Error(), "no owner") {
		t.Errorf("expected error about missing owner, got %q", err.Error())
	}
}

func TestResolveTmuxTarget_OwnerAstra(t *testing.T) {
	// Stub resolveWorkerAgentName to return a known value
	orig := resolveWorkerAgentName
	resolveWorkerAgentName = func(task *taskwarrior.Task) string {
		return CoderAgentName
	}
	defer func() { resolveWorkerAgentName = orig }()

	task := &taskwarrior.Task{
		UUID:        "abc12345-0000-0000-0000-000000000000",
		Description: "test task",
		Project:     "ttal",
		Owner:       "astra",
		Tags:        []string{"feature"},
	}

	target, err := ResolveTmuxTarget(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSession := "ttal-default-astra"
	if target.Session != wantSession {
		t.Errorf("Session = %q, want %q", target.Session, wantSession)
	}
	if target.Window != "coder" {
		t.Errorf("Window = %q, want %q", target.Window, "coder")
	}
	// WorkDir follows the <hex[:8]>-<project> pattern
	wantHex := task.UUID[:8]
	wantProject := "ttal"
	wantWorkDir := config.WorktreesRoot() + "/" + wantHex + "-" + wantProject
	if target.WorkDir != wantWorkDir {
		t.Errorf("WorkDir = %q, want %q", target.WorkDir, wantWorkDir)
	}
}

func TestResolveTmuxTargetForAgent_OwnerAstraAgentCoder(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "def67890-0000-0000-0000-000000000000",
		Description: "another task",
		Project:     "clawd",
		Owner:       "astra",
	}

	target, err := ResolveTmuxTargetForAgent(task, "coder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target.Session != "ttal-default-astra" {
		t.Errorf("Session = %q, want %q", target.Session, "ttal-default-astra")
	}
	if target.Window != "coder" {
		t.Errorf("Window = %q, want %q", target.Window, "coder")
	}
	wantWorkDir := config.WorktreesRoot() + "/" + task.UUID[:8] + "-" + task.Project
	if target.WorkDir != wantWorkDir {
		t.Errorf("WorkDir = %q, want %q", target.WorkDir, wantWorkDir)
	}
}

func TestResolveTmuxTargetForAgent_NilTask(t *testing.T) {
	_, err := ResolveTmuxTargetForAgent(nil, "coder")
	if err == nil {
		t.Fatal("expected error for nil task")
	}
	if !containsStr(err.Error(), "task is nil") {
		t.Errorf("expected 'task is nil', got %q", err.Error())
	}
}

func TestResolveTmuxTargetForAgent_MissingOwner(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "abc12345-0000-0000-0000-000000000000",
		Description: "test task",
		Project:     "ttal",
	}
	_, err := ResolveTmuxTargetForAgent(task, "coder")
	if err == nil {
		t.Fatal("expected error for missing owner")
	}
	if !containsStr(err.Error(), "no owner") {
		t.Errorf("expected error about missing owner, got %q", err.Error())
	}
}

func TestResolveTmuxTargetForAgent_EmptyAgentName(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:    "abc12345-0000-0000-0000-000000000000",
		Owner:   "astra",
		Project: "ttal",
	}
	_, err := ResolveTmuxTargetForAgent(task, "")
	if err == nil {
		t.Fatal("expected error for empty agent name")
	}
	if !containsStr(err.Error(), "agent name is empty") {
		t.Errorf("expected 'agent name is empty', got %q", err.Error())
	}
}

func TestResolveTmuxTarget_WindowNameNoColon(t *testing.T) {
	orig := resolveWorkerAgentName
	resolveWorkerAgentName = func(task *taskwarrior.Task) string {
		return "coder:bad"
	}
	defer func() { resolveWorkerAgentName = orig }()

	task := &taskwarrior.Task{
		UUID:    "abc12345-0000-0000-0000-000000000000",
		Owner:   "astra",
		Project: "ttal",
	}
	_, err := ResolveTmuxTarget(task)
	if err == nil {
		t.Fatal("expected error for colon in window name")
	}
	if !containsStr(err.Error(), "contains ':'") {
		t.Errorf("expected error about colon, got %q", err.Error())
	}
}

func TestResolveTmuxTargetForAgent_WindowNameNoColon(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:    "abc12345-0000-0000-0000-000000000000",
		Owner:   "astra",
		Project: "ttal",
	}
	_, err := ResolveTmuxTargetForAgent(task, "code:r")
	if err == nil {
		t.Fatal("expected error for colon in window name")
	}
	if !containsStr(err.Error(), "contains ':'") {
		t.Errorf("expected error about colon, got %q", err.Error())
	}
}

// Test that resolveWorkerAgentName falls back to CoderAgentName when pipeline
// config is unavailable (e.g. no pipelines.toml). Use the real function without
// stubs to verify the default.
func TestResolveWorkerAgentName_Fallback(t *testing.T) {
	task := &taskwarrior.Task{
		Tags: []string{"nonexistent"},
	}
	name := resolveWorkerAgentName(task)
	if name != CoderAgentName {
		t.Errorf("expected fallback to %q, got %q", CoderAgentName, name)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && containsStrInner(s, substr)
}

func containsStrInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
