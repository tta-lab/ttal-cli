package open

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestResolveAgentSession(t *testing.T) {
	// Create a temp team dir with a stub agent .md file for positive tests.
	teamDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(teamDir, "astra.md"), []byte("---\nrole: designer\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		tags      []string
		teamName  string
		teamPath  string
		wantName  string
		wantFound bool
	}{
		{
			name:      "no tags",
			tags:      nil,
			teamName:  "default",
			teamPath:  teamDir,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "non-agent tags only",
			tags:      []string{"feature", "planned"},
			teamName:  "default",
			teamPath:  teamDir,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "agent tag found",
			tags:      []string{"feature", "astra"},
			teamName:  "default",
			teamPath:  teamDir,
			wantName:  "ttal-default-astra",
			wantFound: true,
		},
		{
			name:      "agent tag first match wins",
			tags:      []string{"astra", "feature"},
			teamName:  "guion",
			teamPath:  teamDir,
			wantName:  "ttal-guion-astra",
			wantFound: true,
		},
		{
			name:      "empty team path",
			tags:      []string{"astra"},
			teamName:  "default",
			teamPath:  "",
			wantName:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &taskwarrior.Task{Tags: tt.tags}
			name, found := resolveAgentSession(task, tt.teamName, tt.teamPath)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}
