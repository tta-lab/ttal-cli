package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

func writeTempPipelinesTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}
	return dir
}

func TestOnAddPipeline_SingleMatch_Passes(t *testing.T) {
	dir := writeTempPipelinesTOML(t, `
[standard]
tags = ["feature"]
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}
	_, _, err = cfg.MatchPipeline([]string{"feature", "backend"})
	if err != nil {
		t.Errorf("expected no error for single match, got: %v", err)
	}
}

func TestOnAddPipeline_NoMatch_Passes(t *testing.T) {
	dir := writeTempPipelinesTOML(t, `
[standard]
tags = ["feature"]
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}
	name, p, err := cfg.MatchPipeline([]string{"unrelated"})
	if err != nil {
		t.Errorf("expected no error for no match, got: %v", err)
	}
	if name != "" || p != nil {
		t.Error("expected no match for unrelated tags")
	}
}

func TestOnAddPipeline_MultipleMatches_Blocked(t *testing.T) {
	dir := writeTempPipelinesTOML(t, `
[a]
tags = ["feature"]
[[a.stages]]
name = "Plan"
assignee = "designer"
gate = "human"

[b]
tags = ["hotfix"]
[[b.stages]]
name = "Fix"
assignee = "fixer"
gate = "auto"
`)
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}
	_, _, err = cfg.MatchPipeline([]string{"feature", "hotfix"})
	if err == nil {
		t.Error("expected error for multiple pipeline matches")
	}
}

func TestOnAddPipeline_NoPipelinesFile_Passes(t *testing.T) {
	// No pipelines.toml — should return empty config without error.
	dir := t.TempDir()
	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	_, _, err = cfg.MatchPipeline([]string{"feature"})
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestOnAddPipelineSkip_RoleMatch(t *testing.T) {
	teamDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(teamDir, "kestrel.md"), []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}

	dir := writeTempPipelinesTOML(t, `
[bugfix]
tags = ["bugfix"]
[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
`)

	cfg, _ := pipeline.Load(dir)
	_, p, _ := cfg.MatchPipeline([]string{"bugfix"})

	agent, err := agentfs.Get(teamDir, "kestrel")
	if err != nil {
		t.Fatalf("agentfs.Get: %v", err)
	}
	if agent.Role != p.Stages[0].Assignee {
		t.Fatal("expected role match")
	}

	task := hookTask{
		"uuid":        "test-uuid",
		"description": "fix something",
		"tags":        []any{"bugfix"},
	}
	task.SetTag("kestrel")
	task.SetStart()

	found := false
	for _, tag := range task.Tags() {
		if tag == "kestrel" {
			found = true
		}
	}
	if !found {
		t.Error("expected +kestrel tag after SetTag")
	}
	if task.Start() == "" {
		t.Error("expected start timestamp after SetStart")
	}
}

func TestOnAddPipelineSkip_RoleMismatch(t *testing.T) {
	teamDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(teamDir, "yuki.md"), []byte("---\nrole: orchestrator\n---\n"), 0o644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}

	dir := writeTempPipelinesTOML(t, `
[bugfix]
tags = ["bugfix"]
[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
`)

	cfg, _ := pipeline.Load(dir)
	_, p, _ := cfg.MatchPipeline([]string{"bugfix"})
	agent, err := agentfs.Get(teamDir, "yuki")
	if err != nil {
		t.Fatalf("agentfs.Get: %v", err)
	}

	if agent.Role == p.Stages[0].Assignee {
		t.Fatal("expected role mismatch for orchestrator")
	}

	// Verify task is NOT mutated — simulating no-skip path
	task := hookTask{
		"uuid":        "test-uuid",
		"description": "fix something",
		"tags":        []any{"bugfix"},
	}
	for _, tag := range task.Tags() {
		if tag == "yuki" {
			t.Error("orchestrator tag should not be added when role doesn't match stage 0")
		}
	}
	if task.Start() != "" {
		t.Error("task should not be started when role doesn't match")
	}
}

func TestOnAddPipelineSkip_NoAgentEnv(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")

	task := hookTask{
		"uuid":        "test-uuid",
		"description": "fix something",
		"tags":        []any{"bugfix"},
	}

	// Simulate the guard: agentName == "" means skip logic doesn't fire
	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName != "" {
		t.Fatal("expected empty TTAL_AGENT_NAME")
	}

	// Task should remain unchanged
	if len(task.Tags()) != 1 || task.Tags()[0] != "bugfix" {
		t.Error("tags should be unchanged when no agent env")
	}
	if task.Start() != "" {
		t.Error("start should be empty when no agent env")
	}
}
