package worker

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

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

func TestTryAutoAdvanceStage0_FixerMatchesStage0(t *testing.T) {
	teamDir := t.TempDir()
	os.MkdirAll(filepath.Join(teamDir, "kestrel"), 0o755) //nolint:errcheck
	kestrelMd := filepath.Join(teamDir, "kestrel", "AGENTS.md")
	if err := os.WriteFile(kestrelMd, []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
	t.Setenv("TTAL_AGENT_NAME", "kestrel")

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

	task := hookTask{
		"uuid":        "test-uuid",
		"description": "fix something",
		"tags":        []any{"bugfix"},
	}
	tryAutoAdvanceStage0(task, p, teamDir)

	tags := task.Tags()
	if !slices.Contains(tags, "kestrel") {
		t.Error("expected +kestrel agent tag")
	}
	if !slices.Contains(tags, "fix") {
		t.Error("expected +fix stage tag")
	}
	if task.Start() == "" {
		t.Error("expected start timestamp")
	}
}

func TestTryAutoAdvanceStage0_OrchestratorNoMatch(t *testing.T) {
	teamDir := t.TempDir()
	os.MkdirAll(filepath.Join(teamDir, "yuki"), 0o755) //nolint:errcheck
	content := []byte("---\nrole: orchestrator\n---\n")
	if err := os.WriteFile(filepath.Join(teamDir, "yuki", "AGENTS.md"), content, 0o644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
	t.Setenv("TTAL_AGENT_NAME", "yuki")

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

	task := hookTask{
		"uuid":        "test-uuid",
		"description": "fix something",
		"tags":        []any{"bugfix"},
	}
	tryAutoAdvanceStage0(task, p, teamDir)

	tags := task.Tags()
	if slices.Contains(tags, "yuki") {
		t.Error("agent tag should NOT be added when role doesn't match")
	}
	if slices.Contains(tags, "fix") {
		t.Error("stage tag should NOT be added when role doesn't match")
	}
	if task.Start() != "" {
		t.Error("start should NOT be set when role doesn't match")
	}
}

func TestTryAutoAdvanceStage0_NoAgentEnv(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")

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

	task := hookTask{
		"uuid":        "test-uuid",
		"description": "fix something",
		"tags":        []any{"bugfix"},
	}
	tryAutoAdvanceStage0(task, p, "")

	if task.Start() != "" {
		t.Error("nothing should be mutated without TTAL_AGENT_NAME")
	}
}
