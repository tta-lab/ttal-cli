package pr

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

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
