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
		path, err := resolveProjectPathOrErrorWithStore("ttal", store)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if path != pathTtal {
			t.Errorf("want %q, got %q", pathTtal, path)
		}
	})

	t.Run("empty project name returns task-has-no-project error", func(t *testing.T) {
		// Use a multi-project store so single-project shortcut doesn't fire
		store := newTestStoreWithProjects(t, []Project{
			{Alias: "ttal", Path: pathTtal},
			{Alias: "other", Path: "/path/other"},
		})
		_, err := resolveProjectPathOrErrorWithStore("", store)
		if err == nil {
			t.Fatal("expected error for empty project name")
		}
		if !strings.Contains(err.Error(), "no project field") {
			t.Errorf("expected 'no project field' error, got: %v", err)
		}
	})

	t.Run("unknown project returns error listing available projects", func(t *testing.T) {
		store := newTestStoreWithProjects(t, []Project{
			{Alias: "ttal", Path: pathTtal},
			{Alias: "flicknote", Path: "/path/flicknote"},
		})
		_, err := resolveProjectPathOrErrorWithStore("nonexistent", store)
		if err == nil {
			t.Fatal("expected error for unknown project")
		}
		msg := err.Error()
		if !strings.Contains(msg, "nonexistent") {
			t.Errorf("error should mention alias: %v", msg)
		}
		if !strings.Contains(msg, "ttal") || !strings.Contains(msg, "flicknote") {
			t.Errorf("error should list available projects: %v", msg)
		}
		if !strings.Contains(msg, "ttal project list") {
			t.Errorf("error should include remediation hint: %v", msg)
		}
	})

	t.Run("hierarchical alias uses base in error message", func(t *testing.T) {
		// "ttal.pr" is not registered; error should mention "ttal" not "ttal.pr".
		// Two projects prevent the single-project shortcut from firing.
		store := newTestStoreWithProjects(t, []Project{
			{Alias: "other", Path: "/path/other"},
			{Alias: "another", Path: "/path/another"},
		})
		_, err := resolveProjectPathOrErrorWithStore("ttal.pr", store)
		if err == nil {
			t.Fatal("expected error for unregistered hierarchical alias")
		}
		msg := err.Error()
		if !strings.Contains(msg, `"ttal"`) {
			t.Errorf("error should use base alias 'ttal', got: %v", msg)
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

func TestResolveProjectAliasWithStore(t *testing.T) {
	const testAlias = "proj"

	t.Run("exact path match", func(t *testing.T) {
		storeDir := t.TempDir()
		workDir := filepath.Join(storeDir, "code")
		store := NewStore(filepath.Join(storeDir, "projects.toml"))
		if err := store.Add(testAlias, testAlias, workDir); err != nil {
			t.Fatalf("Add error: %v", err)
		}
		got := resolveProjectAliasWithStore(workDir, store)
		if got != testAlias {
			t.Errorf("got %q, want %q", got, testAlias)
		}
	})

	t.Run("nested inside registered path", func(t *testing.T) {
		storeDir := t.TempDir()
		projPath := filepath.Join(storeDir, "code")
		subDir := filepath.Join(projPath, "backend", "cmd")
		store := NewStore(filepath.Join(storeDir, "projects.toml"))
		if err := store.Add(testAlias, testAlias, projPath); err != nil {
			t.Fatalf("Add error: %v", err)
		}
		got := resolveProjectAliasWithStore(subDir, store)
		if got != testAlias {
			t.Errorf("got %q, want %q", got, testAlias)
		}
	})

	t.Run("unregistered path", func(t *testing.T) {
		storeDir := t.TempDir()
		workDir := filepath.Join(storeDir, "unregistered")
		store := NewStore(filepath.Join(storeDir, "projects.toml"))
		if err := store.Add(testAlias, testAlias, filepath.Join(storeDir, "other")); err != nil {
			t.Fatalf("Add error: %v", err)
		}
		got := resolveProjectAliasWithStore(workDir, store)
		if got != "" {
			t.Errorf("got %q, want %q", got, "")
		}
	})

	t.Run("store error returns empty", func(t *testing.T) {
		store := NewStore("/nonexistent/projects.toml")
		got := resolveProjectAliasWithStore("/any/path", store)
		if got != "" {
			t.Errorf("got %q, want %q", got, "")
		}
	})
}
