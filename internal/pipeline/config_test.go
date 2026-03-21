package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

const validTOML = `
[standard]
description = "Plan → Implement"
tags = ["feature", "refactor"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-reviewer"
mode = "subagent"

[[standard.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
mode = "subagent"

[bugfix]
description = "Fix → Implement"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
mode = "subagent"
`

func writeTempTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelines.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp toml: %v", err)
	}
	return dir
}

func TestLoad_ValidConfig(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.Pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(cfg.Pipelines))
	}
	std, ok := cfg.Pipelines["standard"]
	if !ok {
		t.Fatal("missing 'standard' pipeline")
	}
	if len(std.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(std.Stages))
	}
}

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(cfg.Pipelines) != 0 {
		t.Errorf("expected empty pipelines, got %d", len(cfg.Pipelines))
	}
}

func TestLoad_ModeDefaultsToSubagent(t *testing.T) {
	const noModeTOML = `
[hotfix]
tags = ["hotfix"]

[[hotfix.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
`
	dir := writeTempTOML(t, noModeTOML)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	stage := cfg.Pipelines["hotfix"].Stages[0]
	const wantMode = "subagent"
	if stage.Mode != wantMode {
		t.Errorf("expected mode %q, got %q", wantMode, stage.Mode)
	}
}

func TestValidate_EmptyStages(t *testing.T) {
	const toml = `
[empty]
tags = ["feature"]
`
	dir := writeTempTOML(t, toml)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for pipeline with no stages")
	}
}

func TestValidate_EmptyTags(t *testing.T) {
	const toml = `
[notags]

[[notags.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
`
	dir := writeTempTOML(t, toml)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for pipeline with no tags")
	}
}

func TestValidate_InvalidGate(t *testing.T) {
	const toml = `
[bad]
tags = ["bad"]

[[bad.stages]]
name = "Plan"
assignee = "designer"
gate = "maybe"
`
	dir := writeTempTOML(t, toml)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid gate value")
	}
}

func TestValidate_OverlappingTagFilters(t *testing.T) {
	const toml = `
[a]
tags = ["feature"]

[[a.stages]]
name = "Plan"
assignee = "designer"
gate = "human"

[b]
tags = ["feature"]

[[b.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
`
	dir := writeTempTOML(t, toml)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for overlapping tag filters")
	}
}

func TestMatchPipeline_SingleMatch(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	name, p, err := cfg.MatchPipeline([]string{"feature", "backend"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "standard" {
		t.Errorf("expected 'standard', got %q", name)
	}
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}
}

func TestMatchPipeline_NoMatch(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	name, p, err := cfg.MatchPipeline([]string{"unrelated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" || p != nil {
		t.Errorf("expected no match, got name=%q, p=%v", name, p)
	}
}

func TestMatchPipeline_EmptyTags(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	name, p, err := cfg.MatchPipeline([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" || p != nil {
		t.Error("expected no match for empty tags")
	}
}

func TestCurrentStage_FindsCorrectStage(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	p := cfg.Pipelines["standard"]
	agentRoles := map[string]string{"inke": "designer", "athena": "researcher"}

	idx, stage, err := p.CurrentStage([]string{"feature", "inke"}, agentRoles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Errorf("expected stage index 0, got %d", idx)
	}
	if stage == nil || stage.Name != "Plan" {
		t.Errorf("expected stage 'Plan', got %v", stage)
	}
}

func TestCurrentStage_NoAgentTag_ReturnsNegativeOne(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	p := cfg.Pipelines["standard"]
	agentRoles := map[string]string{"inke": "designer"}

	idx, stage, err := p.CurrentStage([]string{"feature"}, agentRoles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != -1 || stage != nil {
		t.Errorf("expected (-1, nil), got (%d, %v)", idx, stage)
	}
}

// TestCurrentStage_WorkerStage verifies that without a +worker tag,
// the worker stage is not detected — only the explicit +worker tag
// (added by advanceToStage) makes the stage visible to CurrentStage.
func TestCurrentStage_WorkerStage(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	p := cfg.Pipelines["standard"]
	// Without +worker tag, CurrentStage can't detect the worker stage.
	// The +worker tag is added by advanceToStage when spawning a worker.
	agentRoles := map[string]string{"inke": "designer"}
	idx, stage, err := p.CurrentStage([]string{"feature"}, agentRoles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No agent tag on task → not in any stage yet.
	if idx != -1 || stage != nil {
		t.Errorf("expected (-1, nil) when no agent tag present, got (%d, %v)", idx, stage)
	}
}

func TestCurrentStage_WorkerTag(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "worker", Gate: "auto"},
		},
	}
	agentRoles := map[string]string{"inke": "designer"}

	// +worker tag should match the Implement stage.
	idx, stage, err := p.CurrentStage([]string{"feature", "worker"}, agentRoles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Errorf("expected stage index 1, got %d", idx)
	}
	if stage.Name != "Implement" {
		t.Errorf("expected stage 'Implement', got %q", stage.Name)
	}
}

func TestCurrentStage_WorkerAndAgentTag_Ambiguity(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "worker", Gate: "auto"},
		},
	}
	agentRoles := map[string]string{"inke": "designer"}

	// Both +inke and +worker → ambiguity error.
	_, _, err := p.CurrentStage([]string{"feature", "inke", "worker"}, agentRoles)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}
