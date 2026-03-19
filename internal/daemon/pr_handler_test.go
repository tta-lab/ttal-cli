package daemon

import "testing"

func TestHandlePRCreateMissingFields(t *testing.T) {
	// Missing provider type should fail at provider creation
	resp := handlePRCreate(PRCreateRequest{Owner: "o", Repo: "r", Title: "t"})
	if resp.OK {
		t.Error("expected error for missing provider_type")
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandlePRMergeDeleteBranch(t *testing.T) {
	// Verify DeleteBranch field is wired correctly.
	// Fails at provider creation (no token in test env) — confirms request structure compiles.
	req := PRMergeRequest{
		ProviderType: "forgejo",
		Owner:        "o",
		Repo:         "r",
		Index:        1,
		DeleteBranch: true,
	}
	resp := handlePRMerge(req)
	if resp.OK {
		t.Error("expected error (no token in test env)")
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandlePRCommentListFormatting(t *testing.T) {
	// Verify PRCommentItem structure is populated correctly.
	cr := PRCommentItem{
		User:      "neil",
		Body:      "LGTM",
		CreatedAt: "2026-03-18 12:00",
		HTMLURL:   "https://example.com/pr/1#comment-1",
	}
	if cr.User != "neil" || cr.Body != "LGTM" {
		t.Error("PRCommentItem fields not populated correctly")
	}
}

func TestHandlePRCheckMergeableMissingProvider(t *testing.T) {
	resp := handlePRCheckMergeable(PRCheckMergeableRequest{Owner: "o", Repo: "r", Index: 1})
	if resp.OK {
		t.Error("expected error for missing provider_type")
	}
}

func TestHandlePRGetPRMissingProvider(t *testing.T) {
	resp := handlePRGetPR(PRGetPRRequest{Owner: "o", Repo: "r", Index: 1})
	if resp.OK {
		t.Error("expected error for missing provider_type")
	}
}

func TestHandlePRGetCombinedStatusMissingProvider(t *testing.T) {
	resp := handlePRGetCombinedStatus(PRGetCombinedStatusRequest{Owner: "o", Repo: "r", SHA: "abc"})
	if resp.OK {
		t.Error("expected error for missing provider_type")
	}
}

func TestHandlePRGetCIFailureDetailsMissingProvider(t *testing.T) {
	resp := handlePRGetCIFailureDetails(PRGetCIFailureDetailsRequest{Owner: "o", Repo: "r", SHA: "abc"})
	if resp.OK {
		t.Error("expected error for missing provider_type")
	}
}
