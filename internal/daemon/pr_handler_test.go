package daemon

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
)

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

func TestBuildPRStatusLines_AllFailing(t *testing.T) {
	statuses := []*gitprovider.CommitStatus{
		{Context: "test/unit", State: gitprovider.StateFailure, Description: "5 failures"},
		{Context: "test/lint", State: gitprovider.StateError, Description: "lint error"},
	}
	result := buildPRStatusLines(statuses, 2, 0)
	if !strings.Contains(result, "2 CI check(s) failed") {
		t.Errorf("expected failure count, got: %s", result)
	}
	if !strings.Contains(result, "test/unit") || !strings.Contains(result, "test/lint") {
		t.Errorf("expected check names in output, got: %s", result)
	}
}

func TestBuildPRStatusLines_HasStatusesButNoFailures(t *testing.T) {
	// diagnosePRMergeFailure intercepts the all-pending case before calling buildPRStatusLines,
	// so buildPRStatusLines is only called with failing=0,pending=0 when all checks are
	// success/neutral (not truly pending). Passing a pending status here simulates a
	// success-only slice from the caller's perspective — the statuses slice isn't filtered.
	statuses := []*gitprovider.CommitStatus{
		{Context: "test/unit", State: gitprovider.StatePending},
	}
	result := buildPRStatusLines(statuses, 0, 0)
	if !strings.Contains(result, "All CI checks passed") {
		t.Errorf("expected 'All CI checks passed', got: %s", result)
	}
}

func TestBuildPRStatusLines_NoChecks(t *testing.T) {
	result := buildPRStatusLines(nil, 0, 0)
	if !strings.Contains(result, "No CI checks found") {
		t.Errorf("expected 'No CI checks found', got: %s", result)
	}
}

func TestBuildPRStatusLines_MixedFailingAndPending(t *testing.T) {
	statuses := []*gitprovider.CommitStatus{
		{Context: "test/unit", State: gitprovider.StateFailure, Description: "failed"},
		{Context: "test/e2e", State: gitprovider.StatePending},
	}
	result := buildPRStatusLines(statuses, 1, 1)
	if !strings.Contains(result, "1 CI check(s) failed") {
		t.Errorf("expected failure count, got: %s", result)
	}
	if !strings.Contains(result, "still pending") {
		t.Errorf("expected pending note, got: %s", result)
	}
}

func TestBuildPRStatusLines_WithTargetURL(t *testing.T) {
	statuses := []*gitprovider.CommitStatus{
		{
			Context:     "test/unit",
			State:       gitprovider.StateFailure,
			Description: "failed",
			TargetURL:   "https://ci.example.com/build/42",
		},
	}
	result := buildPRStatusLines(statuses, 1, 0)
	if !strings.Contains(result, "https://ci.example.com/build/42") {
		t.Errorf("expected target URL in output, got: %s", result)
	}
}

func TestCountPRCheckStates(t *testing.T) {
	statuses := []*gitprovider.CommitStatus{
		{State: gitprovider.StateFailure},
		{State: gitprovider.StateError},
		{State: gitprovider.StatePending},
		{State: gitprovider.StateSuccess},
	}
	failing, pending := countPRCheckStates(statuses)
	if failing != 2 {
		t.Errorf("expected 2 failing, got %d", failing)
	}
	if pending != 1 {
		t.Errorf("expected 1 pending, got %d", pending)
	}
}
