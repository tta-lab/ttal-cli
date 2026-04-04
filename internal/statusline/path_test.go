package statusline

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/project"
)

// setupStore creates a temporary projects.toml with a single test project entry.
func setupStore(t *testing.T, alias, projectPath string) (*project.Store, string) {
	t.Helper()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "projects.toml")
	content := fmt.Sprintf("[%s]\nname = \"Test Project\"\npath = %q\n", alias, projectPath)
	if err := os.WriteFile(tomlPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}
	return project.NewStore(tomlPath), t.TempDir() // second TempDir is worktreesRoot
}

func TestCompactPathWith(t *testing.T) {
	homeDir := "/Users/neil"

	// Use a synthetic project path that doesn't collide with any real registered project
	projectPath := "/Users/neil/Code/test-org/myapp"

	store, worktreesRoot := setupStore(t, "myapp", projectPath)

	tests := []struct {
		name          string
		cwd           string
		jobID         string
		store         *project.Store
		worktreesRoot string
		homeDir       string
		want          string
	}{
		{
			name:          "exact project path match",
			cwd:           projectPath,
			jobID:         "",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(myapp)",
		},
		{
			name:          "subdir of project returns alias",
			cwd:           projectPath + "/cmd",
			jobID:         "",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(myapp)",
		},
		{
			name:          "worktree path with jobID",
			cwd:           worktreesRoot + "/ab12cd34-myapp",
			jobID:         "ab12cd34",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(myapp - ab12cd34)",
		},
		{
			name:          "worktree path without jobID",
			cwd:           worktreesRoot + "/ab12cd34-myapp",
			jobID:         "",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(myapp)",
		},
		{
			name:          "worktree with hyphenated alias resolves correctly",
			cwd:           worktreesRoot + "/ab12cd34-myapp-pr",
			jobID:         "ab12cd34",
			store:         func() *project.Store { s, _ := setupStore(t, "myapp-pr", projectPath+"-pr"); return s }(),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(myapp-pr - ab12cd34)",
		},
		{
			name:          "worktree alias not in store falls back to path abbreviation",
			cwd:           worktreesRoot + "/ab12cd34-unknown",
			jobID:         "ab12cd34",
			store:         store, // only has "myapp", not "unknown"
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			// worktreesRoot is a TempDir under /private/var/... or /tmp — just ensure no alias match
			want: func() string { return abbreviatePath(worktreesRoot+"/ab12cd34-unknown", homeDir) }(),
		},
		{
			name:          "store error falls back to path abbreviation",
			cwd:           "/Users/neil/Code/test-org/other",
			jobID:         "",
			store:         project.NewStore("/nonexistent/projects.toml"),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "~/C/t/other",
		},
		{
			name:          "non-project path under home abbreviates intermediate dirs",
			cwd:           "/Users/neil/Code/guion-opensource/some-tool",
			jobID:         "",
			store:         project.NewStore("/nonexistent/projects.toml"),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "~/C/g/some-tool",
		},
		{
			name:          "cwd equals home returns ~",
			cwd:           "/Users/neil",
			jobID:         "",
			store:         project.NewStore("/nonexistent/projects.toml"),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "~",
		},
		{
			name:          "single component under home no intermediate to abbreviate",
			cwd:           "/Users/neil/myproject",
			jobID:         "",
			store:         project.NewStore("/nonexistent/projects.toml"),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "~/myproject",
		},
		{
			name:          "path not under home abbreviates intermediate components",
			cwd:           "/tmp/workspace",
			jobID:         "",
			store:         project.NewStore("/nonexistent/projects.toml"),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "/t/workspace",
		},
		{
			name:          "empty cwd returns empty string",
			cwd:           "",
			jobID:         "",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compactPathWith(tc.cwd, tc.jobID, tc.store, tc.worktreesRoot, tc.homeDir)
			if got != tc.want {
				t.Errorf("compactPathWith(%q, %q) = %q; want %q", tc.cwd, tc.jobID, got, tc.want)
			}
		})
	}
}
