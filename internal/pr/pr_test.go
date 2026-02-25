package pr

import (
	"fmt"
	"strings"
	"testing"

	"codeberg.org/clawteam/ttal-cli/internal/gitprovider"
)

// mockProvider implements gitprovider.Provider for testing.
type mockProvider struct {
	combinedStatus *gitprovider.CombinedStatus
	combinedErr    error
}

func (m *mockProvider) CreatePR(_, _, _, _, _, _ string) (*gitprovider.PullRequest, error) {
	return nil, nil
}
func (m *mockProvider) EditPR(_, _ string, _ int64, _, _ string) (*gitprovider.PullRequest, error) {
	return nil, nil
}
func (m *mockProvider) GetPR(_, _ string, _ int64) (*gitprovider.PullRequest, error) {
	return nil, nil
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
				State: "failure",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: "failure", Description: "exit code 1", TargetURL: "https://ci.example.com/1"},
					{Context: "lint", State: "error", Description: "3 issues", TargetURL: ""},
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
				State: "pending",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: "pending", Description: "waiting"},
					{Context: "lint", State: "pending", Description: "waiting"},
				},
			},
			contains: []string{"2 check(s) still pending"},
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
				State: "success",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: "success", Description: "passed"},
				},
			},
			contains: []string{"All CI checks passed", "merge conflicts or branch protection"},
		},
		{
			name: "mixed failing and pending",
			pr:   &gitprovider.PullRequest{HeadSHA: "abc123"},
			status: &gitprovider.CombinedStatus{
				State: "failure",
				Statuses: []*gitprovider.CommitStatus{
					{Context: "ci/build", State: "failure", Description: "failed"},
					{Context: "lint", State: "pending", Description: "waiting"},
					{Context: "test", State: "success", Description: "passed"},
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
