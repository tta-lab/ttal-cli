package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

// TestResolvePipelinePrompt_NoEnvVars verifies that resolvePipelinePrompt returns empty
// when neither TTAL_JOB_ID nor TTAL_AGENT_NAME is set (no-op path 1).
func TestResolvePipelinePrompt_NoEnvVars(t *testing.T) {
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "")
	got := resolvePipelinePrompt()
	if got != "" {
		t.Errorf("expected empty output with no env vars, got: %q", got)
	}
}

// TestResolvePromptKey_CoderAssignee verifies the coder assignee maps to "coder" prompt key.
func TestResolvePromptKey_CoderAssignee(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "coder")
	stage := &pipeline.Stage{Assignee: "coder", Reviewer: ""}
	got := resolvePromptKey(stage)
	if got != "coder" {
		t.Errorf("resolvePromptKey for coder assignee = %q, want %q", got, "coder")
	}
}

// TestResolvePromptKey_PRReviewer verifies that when TTAL_AGENT_NAME matches stage.Reviewer
// and the assignee is a coder stage, "review" prompt key is returned.
func TestResolvePromptKey_PRReviewer(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "pr-review-lead")
	stage := &pipeline.Stage{Assignee: "coder", Reviewer: "pr-review-lead"}
	got := resolvePromptKey(stage)
	if got != "review" {
		t.Errorf("resolvePromptKey for PR reviewer = %q, want %q", got, "review")
	}
}

// TestResolvePromptKey_PlanReviewer verifies that when TTAL_AGENT_NAME matches stage.Reviewer
// and the assignee is a non-coder stage, "plan_review" prompt key is returned.
func TestResolvePromptKey_PlanReviewer(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "plan-review-lead")
	stage := &pipeline.Stage{Assignee: "designer", Reviewer: "plan-review-lead"}
	got := resolvePromptKey(stage)
	if got != "plan_review" {
		t.Errorf("resolvePromptKey for plan reviewer = %q, want %q", got, "plan_review")
	}
}

// TestResolvePromptKey_DesignerAssignee verifies that designer assignee maps to "designer".
func TestResolvePromptKey_DesignerAssignee(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "mira")
	stage := &pipeline.Stage{Assignee: "designer", Reviewer: ""}
	got := resolvePromptKey(stage)
	if got != "designer" {
		t.Errorf("resolvePromptKey for designer assignee = %q, want %q", got, "designer")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestRenderPipelineGraph_TwoStages(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human", Reviewer: "plan-review-lead"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	if !strings.Contains(out, "Plan [designer]") {
		t.Errorf("expected 'Plan [designer]' in output: %s", out)
	}
	if !strings.Contains(out, "Implement [coder]") {
		t.Errorf("expected 'Implement [coder]' in output: %s", out)
	}
	if !strings.Contains(out, "human/plan-review-lead") {
		t.Errorf("expected 'human/plan-review-lead' in arrow: %s", out)
	}
}

func TestRenderPipelineGraph_SingleStage(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	if !strings.Contains(out, "Implement [coder]") {
		t.Errorf("expected 'Implement [coder]' in output: %s", out)
	}
	// Single stage should have no arrow.
	if strings.Contains(out, "──") {
		t.Errorf("single stage should have no arrow: %s", out)
	}
}

func TestRenderPipelineGraph_NoReviewer(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	// Arrow should show gate only, no reviewer suffix.
	if !strings.Contains(out, "──human──") {
		t.Errorf("expected '──human──' without reviewer in arrow: %s", out)
	}
	if strings.Contains(out, "human/") {
		t.Errorf("should not have reviewer suffix: %s", out)
	}
}

func TestRenderPipelineGraph_WithSkills(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human", Skills: []string{"sp-planning", "flicknote"}},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	if !strings.Contains(out, "(sp-planning, flicknote)") {
		t.Errorf("expected skills in parentheses in output: %s", out)
	}
	// Coder stage has no skills — no trailing parentheses after [coder].
	if strings.Contains(out, "[coder] (") {
		t.Errorf("coder stage should not show skills: %s", out)
	}
}
