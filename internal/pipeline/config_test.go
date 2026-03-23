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

[[standard.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"

[bugfix]
description = "Fix → Implement"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
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

func TestReviewerForStage_ReturnsConfigured(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	// validTOML has standard.stages[0] with assignee=designer and reviewer=plan-reviewer
	name := cfg.ReviewerForStage([]string{"feature"}, "designer")
	if name != "plan-reviewer" {
		t.Errorf("expected 'plan-reviewer', got %q", name)
	}
}

func TestReviewerForStage_NoPipelineMatch(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	name := cfg.ReviewerForStage([]string{"unrelated"}, "designer")
	if name != "" {
		t.Errorf("expected empty string for no pipeline match, got %q", name)
	}
}

func TestReviewerForStage_EmptyReviewerField(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	// validTOML standard.stages[1] has assignee=coder and no reviewer
	name := cfg.ReviewerForStage([]string{"feature"}, "coder")
	if name != "" {
		t.Errorf("expected empty string for stage with no reviewer, got %q", name)
	}
}

func TestReviewerForStage_NoMatchingAssignee(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	// standard pipeline matches "feature" but has no "researcher" assignee
	name := cfg.ReviewerForStage([]string{"feature"}, "researcher")
	if name != "" {
		t.Errorf("expected empty string for no matching assignee, got %q", name)
	}
}

func TestHasReviewer(t *testing.T) {
	const toml = `
[standard]
description = "Plan → Implement"
tags = ["feature"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-review-lead"

[[standard.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
reviewer = "pr-review-lead"
`
	dir := writeTempTOML(t, toml)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	tests := []struct {
		agentName string
		want      bool
	}{
		{"pr-review-lead", true},
		{"plan-review-lead", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		got := cfg.HasReviewer(tt.agentName)
		if got != tt.want {
			t.Errorf("HasReviewer(%q) = %v, want %v", tt.agentName, got, tt.want)
		}
	}
}

func TestReviewerNotifyTarget(t *testing.T) {
	const toml = `
[standard]
description = "Plan → Implement"
tags = ["feature"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-review-lead"

[[standard.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
reviewer = "pr-review-lead"
`
	dir := writeTempTOML(t, toml)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	tests := []struct {
		name      string
		agentName string
		want      NotifyTarget
	}{
		{"worker stage reviewer → coder", "pr-review-lead", NotifyTargetCoder},
		{"non-worker stage reviewer → designer", "plan-review-lead", NotifyTargetDesigner},
		{"not a reviewer → none", "kestrel", NotifyTargetNone},
		{"empty agent name → none", "", NotifyTargetNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ReviewerNotifyTarget(tt.agentName)
			if got != tt.want {
				t.Errorf("ReviewerNotifyTarget(%q) = %v, want %v", tt.agentName, got, tt.want)
			}
		})
	}
}

func TestReviewerNotifyTarget_WorkerWins(t *testing.T) {
	// Agent reviews both worker and non-worker stages → NotifyTargetCoder wins.
	const toml = `
[p1]
description = "Pipeline 1"
tags = ["t1"]

[[p1.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "multi-reviewer"

[p2]
description = "Pipeline 2"
tags = ["t2"]

[[p2.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
reviewer = "multi-reviewer"
`
	dir := writeTempTOML(t, toml)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	got := cfg.ReviewerNotifyTarget("multi-reviewer")
	if got != NotifyTargetCoder {
		t.Errorf("expected NotifyTargetCoder when agent reviews both, got %v", got)
	}
}

func TestStageTag(t *testing.T) {
	s := Stage{Name: "Plan"}
	if got := s.StageTag(); got != "plan" {
		t.Errorf("StageTag() = %q, want %q", got, "plan")
	}
}

func TestStageLGTMTag(t *testing.T) {
	s := Stage{Name: "Implement"}
	if got := s.StageLGTMTag(); got != "implement_lgtm" {
		t.Errorf("StageLGTMTag() = %q, want %q", got, "implement_lgtm")
	}
}

func TestLastStage(t *testing.T) {
	p := Pipeline{Stages: []Stage{{Name: "A"}, {Name: "B"}}}
	if got := p.LastStage(); got.Name != "B" {
		t.Errorf("LastStage() = %q, want %q", got.Name, "B")
	}
}

func TestLastStage_Empty(t *testing.T) {
	p := Pipeline{}
	if got := p.LastStage(); got != nil {
		t.Errorf("LastStage() = %v, want nil", got)
	}
}

func TestValidate_StageNameWithSpace(t *testing.T) {
	const toml = `
[bad]
tags = ["bad"]

[[bad.stages]]
name = "Code Review"
assignee = "designer"
gate = "human"
`
	dir := writeTempTOML(t, toml)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for stage name with space")
	}
}

func TestValidate_StageNameWithHyphen(t *testing.T) {
	const toml = `
[bad]
tags = ["bad"]

[[bad.stages]]
name = "code-review"
assignee = "designer"
gate = "human"
`
	dir := writeTempTOML(t, toml)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for stage name with hyphen")
	}
}

func TestCurrentStage_FindsCorrectStage(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	p := cfg.Pipelines["standard"]
	idx, stage, err := p.CurrentStage([]string{"feature", "plan"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || stage == nil || stage.Name != "Plan" {
		t.Errorf("expected stage 0 (Plan), got %d (%v)", idx, stage)
	}
}

func TestCurrentStage_NoAgentTag_ReturnsNegativeOne(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	p := cfg.Pipelines["standard"]
	idx, stage, err := p.CurrentStage([]string{"feature"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != -1 || stage != nil {
		t.Errorf("expected (-1, nil), got (%d, %v)", idx, stage)
	}
}

func TestCurrentStage_WorkerStage(t *testing.T) {
	dir := writeTempTOML(t, validTOML)
	cfg, _ := Load(dir)

	p := cfg.Pipelines["standard"]
	idx, stage, err := p.CurrentStage([]string{"feature", "plan", "plan_lgtm", "implement"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || stage == nil || stage.Name != "Implement" {
		t.Errorf("expected (1, Implement), got (%d, %v)", idx, stage)
	}
}

func TestCurrentStage_WorkerTag(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	idx, stage, err := p.CurrentStage([]string{"feature", "implement"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || stage.Name != "Implement" {
		t.Errorf("expected stage 1 (Implement), got %d (%q)", idx, stage.Name)
	}
}

func TestCurrentStage_StageTag(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	idx, stage, err := p.CurrentStage([]string{"feature", "plan"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || stage.Name != "Plan" {
		t.Errorf("expected stage 0 (Plan), got %d (%v)", idx, stage)
	}
}

func TestCurrentStage_StageTagWithLGTM_SkipsToNext(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	idx, stage, err := p.CurrentStage([]string{"feature", "plan", "plan_lgtm", "implement"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || stage.Name != "Implement" {
		t.Errorf("expected stage 1 (Implement), got %d (%v)", idx, stage)
	}
}

func TestCurrentStage_AllStagesCompleted(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	idx, stage, err := p.CurrentStage([]string{"feature", "plan", "plan_lgtm", "implement", "implement_lgtm"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || stage.Name != "Implement" {
		t.Errorf("expected last stage (1, Implement), got %d (%v)", idx, stage)
	}
}

func TestCurrentStage_NoStageTag(t *testing.T) {
	p := Pipeline{
		Stages: []Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	idx, stage, err := p.CurrentStage([]string{"feature"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != -1 || stage != nil {
		t.Errorf("expected (-1, nil), got (%d, %v)", idx, stage)
	}
}
