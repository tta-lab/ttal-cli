package daemon

import (
	"context"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	"github.com/tta-lab/ttal-cli/internal/comment"
	"github.com/tta-lab/ttal-cli/internal/ent"
	_ "github.com/tta-lab/ttal-cli/internal/ent/runtime"
)

func newTestCommentService(t *testing.T) *comment.Service {
	t.Helper()
	drv, err := entsql.Open("sqlite", "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB("sqlite3", drv.DB())))
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("schema create: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return comment.NewService(client)
}

func TestHandleCommentAdd_SyncNone_NoMirror(t *testing.T) {
	// sync=none: DB insert succeeds, no mirror even with PRIndex set.
	svc := newTestCommentService(t)
	req := CommentAddRequest{
		Target:       "task-uuid-1",
		Author:       "reviewer",
		Body:         "looks good",
		ProviderType: "forgejo",
		Owner:        "org",
		Repo:         "repo",
		PRIndex:      42,
	}
	resp := handleCommentAdd(svc, "team", "none", req)
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}
	if resp.Round != 1 {
		t.Errorf("want round 1, got %d", resp.Round)
	}
	if resp.ID == "" {
		t.Error("expected non-empty comment ID")
	}
}

func TestHandleCommentAdd_SyncPR_NoPRIndex_NoMirror(t *testing.T) {
	// sync=pr but PRIndex=0: DB insert succeeds, no mirror attempted.
	svc := newTestCommentService(t)
	req := CommentAddRequest{
		Target:       "task-uuid-2",
		Author:       "designer",
		Body:         "plan review findings",
		ProviderType: "",
		Owner:        "",
		Repo:         "",
		PRIndex:      0,
	}
	resp := handleCommentAdd(svc, "team", "pr", req)
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}
	if resp.Round != 1 {
		t.Errorf("want round 1, got %d", resp.Round)
	}
}

func TestHandleCommentAdd_SyncPR_WithPRIndex_MirrorAttempted(t *testing.T) {
	// sync=pr with PRIndex>0: DB insert succeeds, mirror attempted in goroutine.
	// In test env provider creation fails (no token) — confirms code path without panicking.
	svc := newTestCommentService(t)
	req := CommentAddRequest{
		Target:       "task-uuid-3",
		Author:       "coder",
		Body:         "implementation notes",
		ProviderType: "forgejo",
		Owner:        "org",
		Repo:         "repo",
		PRIndex:      7,
	}
	resp := handleCommentAdd(svc, "team", "pr", req)
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}
	if resp.Round != 1 {
		t.Errorf("want round 1, got %d", resp.Round)
	}
	// Allow goroutine to run and log its error (provider creation fails in test env).
	// This is a best-effort check: the goal is "no panic", not deterministic coverage.
	time.Sleep(50 * time.Millisecond)
}
