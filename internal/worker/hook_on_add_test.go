package worker

import (
	"os"
	"path/filepath"
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
