package pr

import (
	"strings"
	"testing"
)

func TestBuildOwnerReviewMessage(t *testing.T) {
	tests := []struct {
		name     string
		prNum    int64
		prURL    string
		title    string
		worktree string
		hex      string
		assignee string
		checks   []string // substrings that must appear
		absences []string // substrings that must NOT appear
	}{
		{
			name:     "happy path",
			prNum:    42,
			prURL:    "https://gh/foo/bar/pull/42",
			title:    "refactor: x",
			worktree: "/Users/neil/.ttal/worktrees/abc12345",
			hex:      "abc12345",
			assignee: "coder",
			checks: []string{
				"PR #42 ready for owner review — task abc12345",
				"ttal go abc12345",
				"ttal send --to abc12345:coder",
				"skill get sp-review-against-plan",
				"commit count/style is cosmetic",
			},
			absences: nil,
		},
		{
			name:     "different assignee",
			prNum:    1,
			prURL:    "https://gh/foo/bar/pull/1",
			title:    "fix: y",
			worktree: "/path/to/worktree",
			hex:      "abc12345",
			assignee: "worker",
			checks: []string{
				"ttal send --to abc12345:worker",
			},
			absences: []string{
				"abc12345:coder",
			},
		},
		{
			name:     "empty title",
			prNum:    3,
			prURL:    "https://gh/foo/bar/pull/3",
			title:    "",
			worktree: "/path/to/worktree",
			hex:      "abc12345",
			assignee: "coder",
			checks: []string{
				"PR #3 ready for owner review — task abc12345",
			},
		},
		{
			name:     "empty worktree",
			prNum:    4,
			prURL:    "https://gh/foo/bar/pull/4",
			title:    "test",
			worktree: "",
			hex:      "abc12345",
			assignee: "coder",
			checks: []string{
				"PR #4 ready for owner review — task abc12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := BuildOwnerReviewMessage(tt.prNum, tt.prURL, tt.title, tt.worktree, tt.hex, tt.assignee)
			for _, check := range tt.checks {
				if !strings.Contains(out, check) {
					t.Errorf("output missing expected substring %q:\n%s", check, out)
				}
			}
			for _, absence := range tt.absences {
				if strings.Contains(out, absence) {
					t.Errorf("output contains unexpected substring %q:\n%s", absence, out)
				}
			}
		})
	}
}
