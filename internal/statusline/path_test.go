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

	// Create a real project path for tests
	projectPath := "/Users/neil/Code/guion-opensource/ttal-cli"

	store, worktreesRoot := setupStore(t, "ttal", projectPath)

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
			want:          "(ttal)",
		},
		{
			name:          "subdir of project returns alias",
			cwd:           projectPath + "/cmd",
			jobID:         "",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(ttal)",
		},
		{
			name:          "worktree path with jobID",
			cwd:           worktreesRoot + "/ab12cd34-ttal",
			jobID:         "ab12cd34",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(ttal - ab12cd34)",
		},
		{
			name:          "worktree path without jobID",
			cwd:           worktreesRoot + "/ab12cd34-ttal",
			jobID:         "",
			store:         store,
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "(ttal)",
		},
		{
			name:          "non-project path under home abbreviates intermediate dirs",
			cwd:           "/Users/neil/Code/guion-opensource/ttal-cli",
			jobID:         "",
			store:         project.NewStore("/nonexistent/projects.toml"),
			worktreesRoot: worktreesRoot,
			homeDir:       homeDir,
			want:          "~/C/g/ttal-cli",
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
