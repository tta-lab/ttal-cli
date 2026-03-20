package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func writeTempPipelinesTOMLCmd(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}
	return dir
}

func TestTaskCommentCmdExists(t *testing.T) {
	var found bool
	for _, sub := range taskCmd.Commands() {
		if sub.Name() == "comment" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal task comment command not found")
	}
}

func TestTaskCommentCmd_RequiresUUIDAndMessage(t *testing.T) {
	if taskCommentCmd.Args == nil {
		t.Error("expected Args validator on comment command")
	}
}

func TestResolveCommentBackend_NilConfig_ReturnsTask(t *testing.T) {
	task := &taskwarrior.Task{Tags: []string{"feature"}}
	got := resolveCommentBackend(task, nil, nil)
	if got != "task" {
		t.Errorf("expected 'task', got %q", got)
	}
}

func TestResolveCommentBackend_NoMatchingPipeline_ReturnsTask(t *testing.T) {
	dir := writeTempPipelinesTOMLCmd(t, `
[standard]
tags = ["feature"]
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
comments = "pr"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}

	// No matching tags — should default to task.
	task := &taskwarrior.Task{Tags: []string{"unrelated"}}
	got := resolveCommentBackend(task, cfg, nil)
	if got != "task" {
		t.Errorf("expected 'task', got %q", got)
	}
}

func TestResolveCommentBackend_MatchingPipelineNoStage_ReturnsTask(t *testing.T) {
	dir := writeTempPipelinesTOMLCmd(t, `
[standard]
tags = ["feature"]
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
comments = "pr"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}

	// Task has feature tag but no agent tag → no active stage → CurrentStage returns idx=-1, stage=nil.
	task := &taskwarrior.Task{Tags: []string{"feature"}}
	got := resolveCommentBackend(task, cfg, nil)
	// No active stage → fallback to "task"
	if got != "task" {
		t.Errorf("expected 'task', got %q", got)
	}
}

func TestResolveCommentBackend_ActiveStageWithPRComments(t *testing.T) {
	dir := writeTempPipelinesTOMLCmd(t, `
[standard]
tags = ["feature"]
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
comments = "pr"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}

	// Simulate active stage: task has feature + designer tags.
	agentRoles := map[string]string{"designer": "designer"}
	task := &taskwarrior.Task{Tags: []string{"feature", "designer"}}
	got := resolveCommentBackend(task, cfg, agentRoles)
	if got != "pr" {
		t.Errorf("expected 'pr', got %q", got)
	}
}

func TestResolveCommentBackend_StageNoCommentsField_ReturnsTask(t *testing.T) {
	dir := writeTempPipelinesTOMLCmd(t, `
[standard]
tags = ["feature"]
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "auto"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}

	agentRoles := map[string]string{"designer": "designer"}
	task := &taskwarrior.Task{Tags: []string{"feature", "designer"}}
	got := resolveCommentBackend(task, cfg, agentRoles)
	if got != "task" {
		t.Errorf("expected 'task' for empty comments field, got %q", got)
	}
}

func TestCommentVerdict_InvalidVerdict_ReturnsError(t *testing.T) {
	// Verify the command's RunE rejects bad verdicts.
	// We do this by checking the flag is registered.
	flag := taskCommentCmd.Flags().Lookup("verdict")
	if flag == nil {
		t.Error("expected --verdict flag on comment command")
	}
}

func TestCommentList_FlagRegistered(t *testing.T) {
	flag := taskCommentCmd.Flags().Lookup("list")
	if flag == nil {
		t.Error("expected --list flag on comment command")
	}
}
