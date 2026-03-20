package open

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAgentSession(t *testing.T) {
	// Create a temp team dir with stub agent .md files for positive tests.
	teamDir := t.TempDir()
	for _, name := range []string{"astra", "inke"} {
		if err := os.WriteFile(filepath.Join(teamDir, name+".md"), []byte("---\nrole: designer\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
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
			// Two valid agent tags: first in slice order wins (astra before inke).
			name:      "agent tag first match wins",
			tags:      []string{"astra", "inke"},
			teamName:  "guion",
			teamPath:  teamDir,
			wantName:  "ttal-guion-astra",
			wantFound: true,
		},
		{
			// Reversed order: inke now comes first.
			name:      "agent tag first match wins reversed",
			tags:      []string{"inke", "astra"},
			teamName:  "guion",
			teamPath:  teamDir,
			wantName:  "ttal-guion-inke",
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
			name, found := ResolveAgentSession(tt.tags, tt.teamName, tt.teamPath)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}
