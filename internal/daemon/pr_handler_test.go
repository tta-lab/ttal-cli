package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
)

// stubProvider is a minimal gitprovider.Provider for unit testing diagnosePRMergeFailure.
// Only GetCombinedStatus is used by that function; all other methods panic.
type stubProvider struct {
	combinedStatus *gitprovider.CombinedStatus
	statusErr      error
}

func (s *stubProvider) Name() string { return "stub" }
func (s *stubProvider) CreatePR(_, _, _, _, _, _ string) (*gitprovider.PullRequest, error) {
	panic("not implemented")
}
func (s *stubProvider) EditPR(_, _ string, _ int64, _, _ string) (*gitprovider.PullRequest, error) {
	panic("not implemented")
}
func (s *stubProvider) GetPR(_, _ string, _ int64) (*gitprovider.PullRequest, error) {
	panic("not implemented")
}
func (s *stubProvider) MergePR(_, _ string, _ int64, _ bool) error { panic("not implemented") }
func (s *stubProvider) CreateComment(_, _ string, _ int64, _ string) (*gitprovider.Comment, error) {
	panic("not implemented")
}
func (s *stubProvider) ListComments(_, _ string, _ int64) ([]*gitprovider.Comment, error) {
	panic("not implemented")
}
func (s *stubProvider) GetCombinedStatus(_, _, _ string) (*gitprovider.CombinedStatus, error) {
	return s.combinedStatus, s.statusErr
}
func (s *stubProvider) GetCIFailureDetails(_, _, _ string) ([]*gitprovider.JobFailure, error) {
	panic("not implemented")
}

// TestDiagnosePRMergeFailure_CIPending exercises the (string, bool) return of
// diagnosePRMergeFailure across the key CI state combinations.
func TestDiagnosePRMergeFailure_CIPending(t *testing.T) {
	cases := []struct {
		name       string
		pr         *gitprovider.PullRequest
		provider   *stubProvider
		wantCI     bool
		wantSubstr string
	}{
		{
			name: "pending only → ciPending=true",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc"},
			provider: &stubProvider{combinedStatus: &gitprovider.CombinedStatus{
				State: "pending",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: gitprovider.StatePending},
				},
			}},
			wantCI:     true,
			wantSubstr: "CI checks still running",
		},
		{
			name: "failing only → ciPending=false",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc"},
			provider: &stubProvider{combinedStatus: &gitprovider.CombinedStatus{
				State: "failure",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/test", State: gitprovider.StateFailure, Description: "tests failed"},
				},
			}},
			wantCI:     false,
			wantSubstr: "CI check(s) failed",
		},
		{
			name: "failing + pending → ciPending=false",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc"},
			provider: &stubProvider{combinedStatus: &gitprovider.CombinedStatus{
				State: "failure",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/test", State: gitprovider.StateFailure, Description: "tests failed"},
					{Context: "ci/lint", State: gitprovider.StatePending},
				},
			}},
			wantCI: false,
		},
		{
			name:       "HeadSHA empty → ciPending=false",
			pr:         &gitprovider.PullRequest{HeadSHA: ""},
			provider:   &stubProvider{},
			wantCI:     false,
			wantSubstr: "Could not determine HEAD SHA",
		},
		{
			name:       "GetCombinedStatus error → ciPending=false",
			pr:         &gitprovider.PullRequest{HeadSHA: "abc"},
			provider:   &stubProvider{statusErr: errors.New("api down")},
			wantCI:     false,
			wantSubstr: "Could not fetch CI status",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, ciPending := diagnosePRMergeFailure(tc.provider, "o", "r", tc.pr)
			if ciPending != tc.wantCI {
				t.Errorf("ciPending = %v, want %v; msg=%q", ciPending, tc.wantCI, msg)
			}
			if tc.wantSubstr != "" && !strings.Contains(msg, tc.wantSubstr) {
				t.Errorf("msg %q missing %q", msg, tc.wantSubstr)
			}
		})
	}
}

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

// TestIsCIPendingMergeError verifies keyword detection for CI-pending merge errors.
func TestIsCIPendingMergeError(t *testing.T) {
	cases := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("some other error"), false},
		{errors.New("failed to merge PR #1: 405 Repository rule violations found." +
			" Required status check is in progress"), true},
		{errors.New("required status check xyz not satisfied"), true},
		{errors.New("merge blocked: status check is in progress"), true},
		{errors.New("merge conflicts"), false},
	}
	for _, tc := range cases {
		got := isCIPendingMergeError(tc.err)
		if got != tc.expected {
			t.Errorf("isCIPendingMergeError(%v) = %v, want %v", tc.err, got, tc.expected)
		}
	}
}

// TestHandlePRMerge_AlreadyMerged verifies that handlePRMerge sets AlreadyMerged=true
// when the provider reports the PR is already merged. This is a structured field check
// that replaced the fragile strings.Contains("already merged") approach in mergeWorkerPR.
func TestHandlePRMerge_AlreadyMerged(t *testing.T) {
	// Minimal Forgejo API mock that handles version discovery and returns a merged PR.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/version"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.21.0"})
		default:
			// GET /api/v1/repos/o/r/pulls/42 — return a merged PR
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number": 42,
				"merged": true,
				"state":  "closed",
			})
		}
	}))
	defer srv.Close()

	t.Setenv("FORGEJO_URL", srv.URL)
	t.Setenv("FORGEJO_TOKEN", "test-token")

	resp := handlePRMerge(PRMergeRequest{
		ProviderType: "forgejo",
		Owner:        "o",
		Repo:         "r",
		Index:        42,
	})

	if resp.OK {
		t.Error("expected not-OK for already merged PR")
	}
	if !resp.AlreadyMerged {
		t.Errorf("expected AlreadyMerged=true, got false; error: %s", resp.Error)
	}
}
