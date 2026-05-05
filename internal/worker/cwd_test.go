package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskHexFromCwd(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{name: "standard worktree path", cwd: filepath.Join(home, ".ttal", "worktrees", "878619a0-ttal"), want: "878619a0"},
		{name: "multi-hyphen alias", cwd: filepath.Join(home, ".ttal", "worktrees", "eb2fde5b-ttal-cli"), want: "eb2fde5b"},
		{name: "empty CWD", cwd: "", want: ""},
		{name: "non-worktree path", cwd: filepath.Join(home, "Code", "project"), want: ""},
		{name: "worktree subdir", cwd: filepath.Join(home, ".ttal", "worktrees", "ec16980f-ttal", "cmd"), want: "ec16980f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskHexFromCwd(tt.cwd)
			if got != tt.want {
				t.Errorf("TaskHexFromCwd(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}
