package pr

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// mockProvider implements gitprovider.Provider for testing.
type mockProvider struct {
	pr             *gitprovider.PullRequest
	prErr          error
	combinedStatus *gitprovider.CombinedStatus
	combinedErr    error
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) CreatePR(_, _, _, _, _, _ string) (*gitprovider.PullRequest, error) {
	return nil, nil
}
func (m *mockProvider) EditPR(_, _ string, _ int64, _, _ string) (*gitprovider.PullRequest, error) {
	return nil, nil
}
func (m *mockProvider) GetPR(_, _ string, _ int64) (*gitprovider.PullRequest, error) {
	return m.pr, m.prErr
}
func (m *mockProvider) MergePR(_, _ string, _ int64, _ bool) error { return nil }
func (m *mockProvider) CreateComment(_, _ string, _ int64, _ string) (*gitprovider.Comment, error) {
	return nil, nil
}
func (m *mockProvider) ListComments(_, _ string, _ int64) ([]*gitprovider.Comment, error) {
	return nil, nil
}
func (m *mockProvider) GetCombinedStatus(_, _, _ string) (*gitprovider.CombinedStatus, error) {
	return m.combinedStatus, m.combinedErr
}
func (m *mockProvider) GetCIFailureDetails(_, _, _ string) ([]*gitprovider.JobFailure, error) {
	return nil, nil
}

func TestDiagnoseMergeFailure(t *testing.T) {
	tests := []struct {
		name      string
		pr        *gitprovider.PullRequest
		status    *gitprovider.CombinedStatus
		statusErr error
		contains  []string
	}{
		{
			name: "empty HeadSHA",
			pr:   &gitprovider.PullRequest{HeadSHA: ""},
			contains: []string{
				"Could not determine HEAD SHA",
				"merge conflicts or branch protection",
			},
		},
		{
			name:      "GetCombinedStatus returns error",
			pr:        &gitprovider.PullRequest{HeadSHA: "abc123"},
			statusErr: fmt.Errorf("API timeout"),
			contains: []string{
				"Could not fetch CI status: API timeout",
				"merge conflicts or branch protection",
			},
		},
		{
			name: "failing checks with target URL",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc123"},
			status: &gitprovider.CombinedStatus{
				State: gitprovider.StateFailure,
				Statuses: []*gitprovider.CommitStatus{
					{
						Context:     "ci/build",
						State:       gitprovider.StateFailure,
						Description: "exit code 1",
						TargetURL:   "https://ci.example.com/1",
					},
					{Context: "lint", State: gitprovider.StateError, Description: "3 issues", TargetURL: ""},
				},
			},
			contains: []string{
				"2 CI check(s) failed",
				"ci/build",
				"exit code 1",
				"https://ci.example.com/1",
				"lint",
				"3 issues",
			},
		},
		{
			name: "pending checks only",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc123"},
			status: &gitprovider.CombinedStatus{
				State: gitprovider.StatePending,
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: gitprovider.StatePending, Description: "waiting"},
					{Context: "lint", State: gitprovider.StatePending, Description: "waiting"},
				},
			},
			contains: []string{
				"CI checks still running (2 pending)",
				"sleep 30 && ttal pr merge",
			},
		},
		{
			name: "no statuses at all",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc123"},
			status: &gitprovider.CombinedStatus{
				State:    "",
				Statuses: nil,
			},
			contains: []string{"No CI checks found", "merge conflicts or branch protection"},
		},
		{
			name: "all checks passing",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc123"},
			status: &gitprovider.CombinedStatus{
				State: gitprovider.StateSuccess,
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: gitprovider.StateSuccess, Description: "passed"},
				},
			},
			contains: []string{"All CI checks passed", "merge conflicts or branch protection"},
		},
		{
			name: "mixed failing and pending",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc123"},
			status: &gitprovider.CombinedStatus{
				State: gitprovider.StateFailure,
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: gitprovider.StateFailure, Description: "failed"},
					{Context: "lint", State: gitprovider.StatePending, Description: "waiting"},
					{Context: "test", State: gitprovider.StateSuccess, Description: "passed"},
				},
			},
			contains: []string{
				"1 CI check(s) failed",
				"ci/build",
				"1 check(s) still pending",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Owner: "owner",
				Repo:  "repo",
				Provider: &mockProvider{
					combinedStatus: tt.status,
					combinedErr:    tt.statusErr,
				},
			}

			result := diagnoseMergeFailure(ctx, tt.pr)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

func TestMergeLGTMGate(t *testing.T) {
	mergeableProvider := &mockProvider{
		pr: &gitprovider.PullRequest{Merged: false, Mergeable: true},
	}

	tests := []struct {
		name        string
		prid        string
		errContains string
	}{
		{
			name:        "blocked without lgtm",
			prid:        "123",
			errContains: "has not been approved by reviewer",
		},
		{
			name:        "empty pr_id returns error",
			prid:        "",
			errContains: "empty pr_id",
		},
		{
			name:        "passes gate with lgtm suffix",
			prid:        "123:lgtm",
			errContains: "", // no error from gate; CheckMergeable succeeds with mergeable PR
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Task:     &taskwarrior.Task{PRID: tt.prid},
				Owner:    "owner",
				Repo:     "repo",
				Provider: mergeableProvider,
			}
			err := Merge(ctx, false)
			if tt.errContains == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.errContains)
				return
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error to contain %q, got: %v", tt.errContains, err)
			}
		})
	}
}

func TestBuildPRURLWithLGTM(t *testing.T) {
	tests := []struct {
		name        string
		prid        string
		wantContain string
		wantEmpty   bool
	}{
		{
			name:        "plain pr_id builds URL",
			prid:        "42",
			wantContain: "42",
		},
		{
			name:        "lgtm pr_id builds correct URL",
			prid:        "42:lgtm",
			wantContain: "42",
		},
		{
			name:      "empty pr_id returns empty",
			prid:      "",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Task: &taskwarrior.Task{PRID: tt.prid},
				Info: &gitprovider.RepoInfo{
					Owner:    "owner",
					Repo:     "repo",
					Provider: "github",
				},
			}
			url := BuildPRURL(ctx)
			if tt.wantEmpty {
				if url != "" {
					t.Errorf("expected empty URL, got %q", url)
				}
				return
			}
			if !strings.Contains(url, tt.wantContain) {
				t.Errorf("expected URL to contain %q, got %q", tt.wantContain, url)
			}
			// Must not contain raw ":lgtm" in URL
			if strings.Contains(url, ":lgtm") {
				t.Errorf("URL must not contain raw :lgtm suffix, got %q", url)
			}
		})
	}
}
