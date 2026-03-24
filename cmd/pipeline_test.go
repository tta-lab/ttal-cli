package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
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
