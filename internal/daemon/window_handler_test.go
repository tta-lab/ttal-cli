package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

const testPipelinesContent = `
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

func writeTempPipelinesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelines.toml")
	if err := os.WriteFile(path, []byte(testPipelinesContent), 0o644); err != nil {
		t.Fatalf("write pipelines fixture: %v", err)
	}
	return dir
}

func TestHandleCloseWindow_RejectsUnknownWindow(t *testing.T) {
	dir := writeTempPipelinesDir(t)
	resp := handleCloseWindowWithConfigDir(CloseWindowRequest{Session: "s", Window: "unknown"}, dir)
	if resp.OK {
		t.Error("expected rejection for unknown window name")
	}
}

func TestHandleCloseWindow_RequiresSession(t *testing.T) {
	dir := writeTempPipelinesDir(t)
	resp := handleCloseWindowWithConfigDir(CloseWindowRequest{Session: "", Window: "pr-review-lead"}, dir)
	if resp.OK {
		t.Error("expected rejection for empty session")
	}
}

func TestHandleCloseWindow_AcceptsReviewWindows(t *testing.T) {
	dir := writeTempPipelinesDir(t)
	// In test env without tmux, WindowExists returns false → idempotent success path.
	for _, window := range []string{"pr-review-lead", "plan-review-lead"} {
		resp := handleCloseWindowWithConfigDir(CloseWindowRequest{Session: "test", Window: window}, dir)
		if !resp.OK {
			t.Errorf("expected OK for window %q, got error: %s", window, resp.Error)
		}
	}
}
