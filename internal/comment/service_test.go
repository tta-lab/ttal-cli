package comment_test

import (
	"context"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	"github.com/tta-lab/ttal-cli/internal/comment"
	"github.com/tta-lab/ttal-cli/internal/ent"
	_ "github.com/tta-lab/ttal-cli/internal/ent/runtime"
)

func newTestService(t *testing.T) *comment.Service {
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

func TestService_Add_RoundIncrement(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	c1, err := svc.Add(ctx, "target-1", "author", "first", "team")
	if err != nil {
		t.Fatalf("Add round 1: %v", err)
	}
	if c1.Round != 1 {
		t.Errorf("first comment: want round 1, got %d", c1.Round)
	}

	c2, err := svc.Add(ctx, "target-1", "author", "second", "team")
	if err != nil {
		t.Fatalf("Add round 2: %v", err)
	}
	if c2.Round != 2 {
		t.Errorf("second comment: want round 2, got %d", c2.Round)
	}
}

func TestService_Add_CrossTargetIsolation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Add(ctx, "target-A", "author", "msg", "team"); err != nil {
		t.Fatalf("Add to target-A: %v", err)
	}

	// target-B should start at round 1, not 2
	c, err := svc.Add(ctx, "target-B", "author", "msg", "team")
	if err != nil {
		t.Fatalf("Add to target-B: %v", err)
	}
	if c.Round != 1 {
		t.Errorf("cross-target: want round 1 for target-B, got %d", c.Round)
	}
}

func TestService_List_OrderedByCreatedAt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	for _, body := range []string{"first", "second", "third"} {
		if _, err := svc.Add(ctx, "target-1", "author", body, "team"); err != nil {
			t.Fatalf("Add %q: %v", body, err)
		}
	}

	comments, err := svc.List(ctx, "target-1", "team")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(comments) != 3 {
		t.Fatalf("want 3 comments, got %d", len(comments))
	}
	if comments[0].Body != "first" || comments[2].Body != "third" {
		t.Errorf("unexpected order: %v %v %v", comments[0].Body, comments[1].Body, comments[2].Body)
	}
}

func TestService_GetByRound(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Add 3 comments
	for _, body := range []string{"round one", "round two", "round three"} {
		if _, err := svc.Add(ctx, "target-1", "author", body, "team"); err != nil {
			t.Fatalf("Add %q: %v", body, err)
		}
	}

	// Get round 2
	comments, err := svc.GetByRound(ctx, "target-1", "team", 2)
	if err != nil {
		t.Fatalf("GetByRound: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("want 1 comment for round 2, got %d", len(comments))
	}
	if comments[0].Body != "round two" {
		t.Errorf("want body %q, got %q", "round two", comments[0].Body)
	}

	// Get non-existent round
	comments, err = svc.GetByRound(ctx, "target-1", "team", 99)
	if err != nil {
		t.Fatalf("GetByRound(99): %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("want 0 comments for round 99, got %d", len(comments))
	}
}

func TestService_GetByRound_ZeroRound(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Add(ctx, "target-1", "author", "msg", "team"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// round=0 should return empty, not the first comment
	comments, err := svc.GetByRound(ctx, "target-1", "team", 0)
	if err != nil {
		t.Fatalf("GetByRound(0): %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("want 0 comments for round 0, got %d", len(comments))
	}
}

func TestService_GetByRound_CrossTeamIsolation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Add a comment for team-A at round 1
	if _, err := svc.Add(ctx, "target-1", "author", "team-a comment", "team-a"); err != nil {
		t.Fatalf("Add team-a: %v", err)
	}
	// Add a comment for team-B at round 1
	if _, err := svc.Add(ctx, "target-1", "author", "team-b comment", "team-b"); err != nil {
		t.Fatalf("Add team-b: %v", err)
	}

	comments, err := svc.GetByRound(ctx, "target-1", "team-a", 1)
	if err != nil {
		t.Fatalf("GetByRound team-a: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("want 1 comment for team-a, got %d", len(comments))
	}
	if comments[0].Body != "team-a comment" {
		t.Errorf("cross-team leak: got %q, want team-a comment", comments[0].Body)
	}
}

func TestService_CurrentRound_EmptyReturnsZero(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	round, err := svc.CurrentRound(ctx, "no-comments", "team")
	if err != nil {
		t.Fatalf("CurrentRound: %v", err)
	}
	if round != 0 {
		t.Errorf("want 0, got %d", round)
	}
}
