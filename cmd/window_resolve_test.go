package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

const testFallbackReviewer = "pr-review-lead"

const resolveTestPipelines = `
[standard]
description = "Plan → Implement"
tags = ["feature", "refactor"]

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

func writeResolvePipelinesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte(resolveTestPipelines), 0o644); err != nil {
		t.Fatalf("write pipelines fixture: %v", err)
	}
	return dir
}

func TestResolveReviewerWindowForDir_FoundInPipeline(t *testing.T) {
	dir := writeResolvePipelinesDir(t)
	got := resolveReviewerWindowForDir(dir, []string{"feature"}, "coder", testFallbackReviewer)
	if got != testFallbackReviewer {
		t.Errorf("expected pr-review-lead, got %q", got)
	}
}

func TestResolveReviewerWindowForDir_NoPipelineMatch_ReturnsFallback(t *testing.T) {
	dir := writeResolvePipelinesDir(t)
	got := resolveReviewerWindowForDir(dir, []string{"unrelated"}, "coder", testFallbackReviewer)
	if got != testFallbackReviewer {
		t.Errorf("expected fallback pr-review-lead, got %q", got)
	}
}

func TestResolveReviewerWindowForDir_NoReviewerConfigured_ReturnsFallback(t *testing.T) {
	// A pipeline with a matching stage but no reviewer field.
	const toml = `
[noreview]
description = "No reviewer"
tags = ["noreview"]

[[noreview.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveReviewerWindowForDir(dir, []string{"noreview"}, "coder", testFallbackReviewer)
	if got != testFallbackReviewer {
		t.Errorf("expected fallback pr-review-lead, got %q", got)
	}
}

func TestResolveReviewerWindowForDir_LoadFailure_ReturnsFallback(t *testing.T) {
	got := resolveReviewerWindowForDir("/nonexistent/path", []string{"feature"}, "coder", testFallbackReviewer)
	if got != testFallbackReviewer {
		t.Errorf("expected fallback on load failure, got %q", got)
	}
}
