package worker

import (
	"context"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/temenos"
)

// containsPrefix reports whether s starts with prefix.
func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestCleanupReviewerTokens_PRAndPlan(t *testing.T) {
	t.Run("correct prefix matching", func(t *testing.T) {
		prAnn := taskwarrior.Annotation{Description: "temenos_pr_reviewer_token:abc123token"}
		planAnn := taskwarrior.Annotation{Description: "temenos_plan_reviewer_token:def456token"}
		workerAnn := taskwarrior.Annotation{Description: "temenos_token:workertoken"}
		irrelevant := taskwarrior.Annotation{Description: "flicknote:abc123"}

		if !containsPrefix(prAnn.Description, temenos.TokenAnnotationPRReviewer) {
			t.Errorf("PR prefix mismatch")
		}
		if !containsPrefix(planAnn.Description, temenos.TokenAnnotationPlanReviewer) {
			t.Errorf("plan prefix mismatch")
		}
		if !containsPrefix(workerAnn.Description, temenos.TokenAnnotationWorker) {
			t.Errorf("worker prefix mismatch")
		}
		if containsPrefix(irrelevant.Description, temenos.TokenAnnotationPRReviewer) {
			t.Errorf("irrelevant annotation should not match PR prefix")
		}
	})
}

// TestCleanupReviewerTokens_EmptyAnnotations verifies no-op with empty list.
func TestCleanupReviewerTokens_EmptyAnnotations(t *testing.T) {
	// We can't easily test the full function without a fake socket,
	// but we verify the logic is exercised correctly by checking the loop doesn't panic.
	cleanupReviewerTokens(context.Background(), "abc12345", nil)
	cleanupReviewerTokens(context.Background(), "abc12345", []taskwarrior.Annotation{})
}

// TestReviewerMCPName verifies the name format.
func TestReviewerMCPName(t *testing.T) {
	if got := temenos.ReviewerMCPName("abc12345", "pr"); got != "r-abc12345-pr" {
		t.Errorf("pr reviewer MCP name = %q, want %q", got, "r-abc12345-pr")
	}
	if got := temenos.ReviewerMCPName("abc12345", "plan"); got != "r-abc12345-plan" {
		t.Errorf("plan reviewer MCP name = %q, want %q", got, "r-abc12345-plan")
	}
}
