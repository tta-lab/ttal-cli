package project

import (
	"encoding/json"
	"testing"
)

// stubProjectBinary sets the shell-out to return the given project as a single resolve result.
func stubProjectBinary(t *testing.T, proj Project) {
	t.Helper()
	orig := runProjectBinary
	SetBinaryFn(func(args ...string) ([]byte, error) {
		return json.Marshal(proj)
	})
	t.Cleanup(func() { runProjectBinary = orig })
}

func TestExtractWorktreeAlias(t *testing.T) {
	origRootFn := worktreesRootFn
	worktreesRootFn = func() string { return "/home/user/.ttal/worktrees" }
	t.Cleanup(func() { worktreesRootFn = origRootFn })

	tests := []struct {
		name, cwd string
		stub      Project
		want      string
	}{
		{"uuid8-alias", "/home/user/.ttal/worktrees/abc12345-fb", Project{Alias: "fb", Path: "/repo/fb"}, "fb"},
		{"subdirectory", "/home/user/.ttal/worktrees/deadbeef-fb/src", Project{Alias: "fb", Path: "/repo/fb"}, "fb"},
		{"hyphens in alias", "/home/user/.ttal/worktrees/12345678-fb-cli", Project{Alias: "fb-cli", Path: "/repo/fb"}, "fb-cli"},
		{"not a worktree path", "/home/user/code/ttal-cli", Project{Alias: "ttal", Path: "/code/ttal-cli"}, ""},
		{"too-short uuid", "/home/user/.ttal/worktrees/abc-fb", Project{Alias: "fb", Path: "/repo/fb"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubProjectBinary(t, tt.stub)
			got := resolveProjectAliasInner(tt.cwd)
			if got != tt.want {
				t.Errorf("resolveProjectAliasInner(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}
