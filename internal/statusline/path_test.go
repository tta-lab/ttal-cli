package statusline

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/project"
)

func TestCompactPathWith(t *testing.T) {
	homeDir := "/Users/neil"

	// Use a synthetic project path that doesn't collide with any real registered project
	projectPath := "/Users/neil/Code/test-org/myapp"

	// Save original and restore
	origResolve := resolveAliasFn
	t.Cleanup(func() { resolveAliasFn = origResolve })

	// Override with a test resolver that matches path prefix
	resolveAliasFn = func(workDir string) string {
		if workDir == projectPath || len(workDir) > len(projectPath) && workDir[:len(projectPath)] == projectPath {
			return "myapp"
		}
		return project.ResolveProjectAlias(workDir)
	}

	tests := []struct {
		name    string
		cwd     string
		jobID   string
		homeDir string
		want    string
	}{
		{
			name:    "exact project path match",
			cwd:     projectPath,
			jobID:   "",
			homeDir: homeDir,
			want:    "(myapp)",
		},
		{
			name:    "subdir of project returns alias",
			cwd:     projectPath + "/cmd",
			jobID:   "",
			homeDir: homeDir,
			want:    "(myapp)",
		},
		{
			name:    "non-project path under home abbreviates intermediate dirs",
			cwd:     "/Users/neil/Code/guion-opensource/some-tool",
			jobID:   "",
			homeDir: homeDir,
			want:    "~/C/g/some-tool",
		},
		{
			name:    "cwd equals home returns ~",
			cwd:     "/Users/neil",
			jobID:   "",
			homeDir: homeDir,
			want:    "~",
		},
		{
			name:    "single component under home no intermediate to abbreviate",
			cwd:     "/Users/neil/myproject",
			jobID:   "",
			homeDir: homeDir,
			want:    "~/myproject",
		},
		{
			name:    "path not under home abbreviates intermediate components",
			cwd:     "/tmp/workspace",
			jobID:   "",
			homeDir: homeDir,
			want:    "/t/workspace",
		},
		{
			name:    "empty cwd returns empty string",
			cwd:     "",
			jobID:   "",
			homeDir: homeDir,
			want:    "",
		},
		{
			name:    "alias with jobID",
			cwd:     projectPath,
			jobID:   "ab12cd34",
			homeDir: homeDir,
			want:    "(myapp - ab12cd34)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compactPathWith(tc.cwd, tc.jobID, tc.homeDir)
			if got != tc.want {
				t.Errorf("compactPathWith(%q, %q) = %q; want %q", tc.cwd, tc.jobID, got, tc.want)
			}
		})
	}
}
