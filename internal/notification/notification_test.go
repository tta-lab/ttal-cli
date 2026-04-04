package notification_test

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/notification"
)

func fullCtx() notification.Context {
	return notification.NewContext("ttal", "eba5d7ab", "standardize notifications", "Implement")
}

func noProjectCtx() notification.Context {
	return notification.NewContext("", "eba5d7ab", "standardize notifications", "Implement")
}

func emptyCtx() notification.Context {
	return notification.NewContext("", "", "do something", "")
}

func TestNewContext(t *testing.T) {
	ctx := notification.NewContext("ttal", "eba5d7ab", "desc", "Implement")
	if ctx.Project != "ttal" || ctx.TaskID != "eba5d7ab" {
		t.Fatalf("unexpected context: %+v", ctx)
	}
}

func TestTaskDone_WithPR(t *testing.T) {
	n := notification.TaskDone{Ctx: fullCtx(), PRIndex: 42}
	got := n.Render()
	want := "✅ [ttal · eba5d7ab] Task done, PR #42 merged — standardize notifications"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestTaskDone_NoPR(t *testing.T) {
	n := notification.TaskDone{Ctx: fullCtx(), PRIndex: 0}
	got := n.Render()
	want := "✅ [ttal · eba5d7ab] Task done — standardize notifications"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestTaskDone_NoProject(t *testing.T) {
	n := notification.TaskDone{Ctx: noProjectCtx(), PRIndex: 1}
	got := n.Render()
	if !strings.HasPrefix(got, "✅ [eba5d7ab]") {
		t.Errorf("expected [eba5d7ab] prefix, got: %q", got)
	}
}

func TestTaskDone_EmptyCtx(t *testing.T) {
	n := notification.TaskDone{Ctx: emptyCtx()}
	got := n.Render()
	if !strings.HasPrefix(got, "✅ ") {
		t.Errorf("expected emoji prefix, got: %q", got)
	}
	if strings.Contains(got, "[]") {
		t.Errorf("unexpected empty brackets: %q", got)
	}
}

func TestPRCreated(t *testing.T) {
	n := notification.PRCreated{
		Ctx:   fullCtx(),
		Title: "Standardize notifications",
		URL:   "https://github.com/owner/repo/pull/1",
	}
	got := n.Render()
	want := `📋 [ttal · eba5d7ab] PR created: "Standardize notifications" — https://github.com/owner/repo/pull/1`
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPRReadyToMerge(t *testing.T) {
	n := notification.PRReadyToMerge{Ctx: fullCtx()}
	got := n.Render()
	want := "🔔 [ttal · eba5d7ab] PR ready to merge — standardize notifications"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPRMergeFailed(t *testing.T) {
	n := notification.PRMergeFailed{
		Ctx:    fullCtx(),
		Reason: "timeout",
	}
	got := n.Render()
	if !strings.Contains(got, "⚠️") || !strings.Contains(got, "timeout") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestPRMergeBlocked(t *testing.T) {
	n := notification.PRMergeBlocked{Ctx: fullCtx(), Reason: "uncommitted changes in worktree"}
	got := n.Render()
	want := "⚠️ [ttal · eba5d7ab] Merge blocked for standardize notifications: uncommitted changes in worktree"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCIFailed(t *testing.T) {
	n := notification.CIFailed{Ctx: fullCtx(), PRIndex: 7, SHA: "abc12345def"}
	got := n.Render()
	if !strings.Contains(got, "❌") || !strings.Contains(got, "PR #7") || !strings.Contains(got, "abc12345") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestCIPassed(t *testing.T) {
	n := notification.CIPassed{Ctx: fullCtx(), PRIndex: 7, SHA: "abc12345def"}
	got := n.Render()
	if !strings.Contains(got, "✅") || !strings.Contains(got, "PR #7") || !strings.Contains(got, "abc12345") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestCIPendingMerge(t *testing.T) {
	n := notification.CIPendingMerge{Ctx: fullCtx(), PRIndex: 7}
	got := n.Render()
	if !strings.Contains(got, "⏳") || !strings.Contains(got, "PR #7") {
		t.Errorf("unexpected output: %q", got)
	}
	if !strings.Contains(got, "CI checks still running") {
		t.Errorf("expected CI-pending message, got: %q", got)
	}
}

func TestMergeConflict(t *testing.T) {
	n := notification.MergeConflict{Ctx: fullCtx(), PRIndex: 5}
	got := n.Render()
	want := "⚠️ [ttal · eba5d7ab] PR #5 has merge conflicts — rebase or merge base branch to resolve."
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCleanupFailed(t *testing.T) {
	n := notification.CleanupFailed{
		Ctx:       fullCtx(),
		SessionID: "worker-abc",
		Err:       "exit status 1",
	}
	got := n.Render()
	if !strings.Contains(got, "⚠️") || !strings.Contains(got, "worker-abc") || !strings.Contains(got, "exit status 1") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestGateRequest_Render(t *testing.T) {
	// fullCtx has Stage="" — use NewContext with a stage for GateRequest
	n := notification.GateRequest{
		Ctx: notification.NewContext("ttal", "eba5d7ab", "standardize notifications", "Implement"),
	}
	got := n.Render()
	if !strings.Contains(got, "🔒") || !strings.Contains(got, "Implement") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestGateRequest_RenderHTML(t *testing.T) {
	n := notification.GateRequest{
		Ctx: notification.NewContext("ttal", "eba5d7ab", "<script>alert('xss')</script>", "Implement"),
	}
	got := n.RenderHTML()
	if strings.Contains(got, "<script>") {
		t.Errorf("HTML not escaped: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag: %q", got)
	}
}

func TestReminder(t *testing.T) {
	n := notification.Reminder{Ctx: notification.NewContext("", "", "deploy staging", "")}
	got := n.Render()
	want := "🔔 deploy staging"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDaemonReady(t *testing.T) {
	n := notification.DaemonReady{}
	got := n.Render()
	want := "✅ Daemon ready"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}
