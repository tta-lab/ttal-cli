package project

import (
	"path/filepath"
	"strings"
	"testing"
)

const pathTtal = "/path/ttal"

// newTestStoreWithProjects creates a temp store pre-populated with the given projects.
func newTestStoreWithProjects(t *testing.T, projects []Project) *Store {
	t.Helper()
	s := NewStore(filepath.Join(t.TempDir(), "projects.toml"))
	for _, p := range projects {
		if err := s.Add(p.Alias, p.Alias, p.Path); err != nil {
			t.Fatalf("Add(%q) error: %v", p.Alias, err)
		}
	}
	return s
}

func TestResolveProjectPathWithStore(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		projects    []Project
		want        string
	}{
		{
			name:        "exact match",
			projectName: "ttal",
			projects:    []Project{{Alias: "ttal", Path: pathTtal}},
			want:        pathTtal,
		},
		{
			name:        "hierarchical fallback: ttal.pr → ttal",
			projectName: "ttal.pr",
			projects:    []Project{{Alias: "ttal", Path: pathTtal}},
			want:        pathTtal,
		},
		{
			name:        "contains fallback: ttal-cli contains ttal",
			projectName: "ttal-cli",
			projects:    []Project{{Alias: "ttal", Path: pathTtal}},
			want:        pathTtal,
		},
		{
			name:        "empty project name — single-project shortcut",
			projectName: "",
			projects:    []Project{{Alias: "ttal", Path: pathTtal}},
			want:        pathTtal,
		},
		{
			name:        "unknown project returns empty",
			projectName: "nonexistent",
			projects:    []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "other", Path: "/path/other"}},
			want:        "",
		},
		{
			name:        "empty project name with multiple projects returns empty",
			projectName: "",
			projects:    []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "other", Path: "/path/other"}},
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStoreWithProjects(t, tt.projects)
			got := resolveProjectPathWithStore(tt.projectName, store)
			if got != tt.want {
				t.Errorf("resolveProjectPathWithStore(%q) = %q, want %q", tt.projectName, got, tt.want)
			}
		})
	}
}

func TestResolveProjectPathOrError(t *testing.T) {
	t.Run("found returns path", func(t *testing.T) {
		store := newTestStoreWithProjects(t, []Project{{Alias: "ttal", Path: pathTtal}})
		// Call resolveProjectPathWithStore directly to avoid needing the real config path
		path := resolveProjectPathWithStore("ttal", store)
		if path != pathTtal {
			t.Errorf("want /path/ttal, got %q", path)
		}
	})

	t.Run("unknown project returns error listing available", func(t *testing.T) {
		store := newTestStoreWithProjects(t, []Project{
			{Alias: "ttal", Path: pathTtal},
			{Alias: "flicknote", Path: "/path/flicknote"},
		})
		err := formatProjectNotFoundError("unknown", store)
		if err == nil {
			t.Fatal("expected error for unknown project")
		}
		msg := err.Error()
		if !strings.Contains(msg, "unknown") {
			t.Errorf("error should mention alias %q: %v", "unknown", msg)
		}
		if !strings.Contains(msg, "ttal") || !strings.Contains(msg, "flicknote") {
			t.Errorf("error should list available projects: %v", msg)
		}
		if !strings.Contains(msg, "ttal project list") {
			t.Errorf("error should include remediation hint: %v", msg)
		}
	})

	t.Run("empty project name returns appropriate error", func(t *testing.T) {
		store := newTestStoreWithProjects(t, []Project{{Alias: "ttal", Path: pathTtal}})
		path := resolveProjectPathWithStore("", store)
		// Empty name with single project triggers single-project shortcut
		if path != pathTtal {
			t.Errorf("single-project shortcut: want /path/ttal, got %q", path)
		}
	})

	t.Run("hierarchical fallback ttal.pr.sub → ttal", func(t *testing.T) {
		store := newTestStoreWithProjects(t, []Project{{Alias: "ttal", Path: pathTtal}})
		path := resolveProjectPathWithStore("ttal.pr.sub", store)
		if path != pathTtal {
			t.Errorf("want /path/ttal for ttal.pr.sub, got %q", path)
		}
	})
}

func TestMatchByContains(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		projects []Project
		want     string
	}{
		{
			name:  "input contains one alias",
			input: "ttal-cli",
			projects: []Project{
				{Alias: "ttal", Path: pathTtal},
				{Alias: "flicknote", Path: "/path/flicknote"},
			},
			want: pathTtal,
		},
		{
			name:  "input contains multiple aliases - ambiguous",
			input: "ttal-flicknote-app",
			projects: []Project{
				{Alias: "ttal", Path: pathTtal},
				{Alias: "flicknote", Path: "/path/flicknote"},
			},
			want: "",
		},
		{
			name:  "alias contains input but not vice versa - no match",
			input: "tt",
			projects: []Project{
				{Alias: "ttal", Path: pathTtal},
			},
			want: "",
		},
		{
			name:  "case insensitive match",
			input: "TTAL-CLI",
			projects: []Project{
				{Alias: "ttal", Path: pathTtal},
			},
			want: pathTtal,
		},
		{
			name:  "empty alias skipped",
			input: "anything",
			projects: []Project{
				{Alias: "", Path: "/path/empty"},
			},
			want: "",
		},
		{
			name:  "project with no path skipped",
			input: "ttal-cli",
			projects: []Project{
				{Alias: "ttal", Path: ""},
			},
			want: "",
		},
		{
			name:     "no projects",
			input:    "ttal-cli",
			projects: nil,
			want:     "",
		},
		{
			name:  "exact alias match via contains",
			input: "ttal",
			projects: []Project{
				{Alias: "ttal", Path: pathTtal},
			},
			want: pathTtal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchByContains(tt.input, tt.projects)
			if got != tt.want {
				t.Errorf("matchByContains(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
