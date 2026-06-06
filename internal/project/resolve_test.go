package project

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func stubProjects(t *testing.T, projects []Project) {
	t.Helper()
	orig := runProjectBinary
	SetBinaryFn(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "resolve" {
			for _, p := range projects {
				if p.Path == args[1] || p.Alias == args[1] {
					return json.Marshal(p)
				}
			}
			return []byte("{}"), nil
		}
		return json.Marshal(projects)
	})
	t.Cleanup(func() { runProjectBinary = orig })
}

func TestExtractWorktreeAlias(t *testing.T) {
	origRootFn := worktreesRootFn
	root := "/home/user/.ttal/worktrees"
	worktreesRootFn = func() string { return root }
	t.Cleanup(func() { worktreesRootFn = origRootFn })

	proj := Project{Alias: "fb", Path: "/repo/fb"}
	repoProj := "/repo/fb"

	tests := []struct {
		name, cwd string
		stub      []Project
		want      string
	}{
		{
			"uuid8-alias",
			filepath.Join(root, "abc12345-fb"),
			[]Project{proj},
			"fb",
		},
		{
			"subdirectory",
			filepath.Join(root, "deadbeef-fb", "src"),
			[]Project{proj},
			"fb",
		},
		{
			"hyphens in alias",
			filepath.Join(root, "12345678-fb-cli"),
			[]Project{{Alias: "fb-cli", Path: repoProj}},
			"fb-cli",
		},
		{
			"unknown alias",
			filepath.Join(root, "abc12345-unknown"),
			[]Project{proj},
			"",
		},
		{
			"too-short uuid",
			filepath.Join(root, "abc-fb"),
			[]Project{proj},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubProjects(t, tt.stub)
			got := resolveProjectAliasInner(tt.cwd)
			if got != tt.want {
				t.Errorf("resolveProjectAliasInner(%q)=%q want %q", tt.cwd, got, tt.want)
			}
		})
	}
}
