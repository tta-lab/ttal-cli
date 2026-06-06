package project

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const pathTtal = "/path/ttal"

func stubProjects(t *testing.T, projects []Project) {
	t.Helper()
	orig := runProjectBinary
	SetBinaryFn(func(args ...string) ([]byte, error) {
		return json.Marshal(projects)
	})
	t.Cleanup(func() { runProjectBinary = orig })
}

func TestResolveProjectPath(t *testing.T) {
	tests := []struct {
		name, projectName string
		projects          []Project
		want              string
	}{
		{"exact", "ttal", []Project{{Alias: "ttal", Path: pathTtal}}, pathTtal},
		{"hierarchical", "ttal.pr", []Project{{Alias: "ttal", Path: pathTtal}}, pathTtal},
		{"contains", "ttal-cli", []Project{{Alias: "ttal", Path: pathTtal}}, pathTtal},
		{"empty single", "", []Project{{Alias: "ttal", Path: pathTtal}}, pathTtal},
		{"unknown", "x", []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "o", Path: "/o"}}, ""},
		{"empty multi", "", []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "o", Path: "/o"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubProjects(t, tt.projects)
			if got := ResolveProjectPath(tt.projectName); got != tt.want {
				t.Errorf("ResolveProjectPath(%q)=%q want %q", tt.projectName, got, tt.want)
			}
		})
	}
}

func TestResolveProjectPathOrError(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		stubProjects(t, []Project{{Alias: "ttal", Path: pathTtal}})
		p, err := ResolveProjectPathOrError("ttal")
		if err != nil || p != pathTtal {
			t.Fatalf("got %q,%v want %q,nil", p, err, pathTtal)
		}
	})
	t.Run("empty", func(t *testing.T) {
		stubProjects(t, []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "o", Path: "/o"}})
		_, err := ResolveProjectPathOrError("")
		if err == nil || !strings.Contains(err.Error(), "no project field") {
			t.Fatalf("unexpected: %v", err)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		stubProjects(t, []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "fn", Path: "/f"}})
		_, err := ResolveProjectPathOrError("x")
		if err == nil {
			t.Fatal("want error")
		}
		s := err.Error()
		if !strings.Contains(s, "x") || !strings.Contains(s, "ttal") || !strings.Contains(s, "project list") {
			t.Errorf("bad: %v", err)
		}
	})
	t.Run("hierarchical base", func(t *testing.T) {
		stubProjects(t, []Project{{Alias: "o", Path: "/o"}, {Alias: "a", Path: "/a"}})
		_, err := ResolveProjectPathOrError("ttal.pr")
		if err == nil || !strings.Contains(err.Error(), `"ttal"`) {
			t.Errorf("%v", err)
		}
	})
}

func TestMatchProjectPathByContains(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		projects []Project
		want     string
	}{
		{"one", "ttal-cli", []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "f", Path: "/f"}}, pathTtal},
		{"ambig", "ttal-f", []Project{{Alias: "ttal", Path: pathTtal}, {Alias: "f", Path: "/f"}}, ""},
		{"case", "TTAL-CLI", []Project{{Alias: "ttal", Path: pathTtal}}, pathTtal},
		{"empty alias", "x", []Project{{Alias: "", Path: "/e"}}, ""},
		{"empty path", "ttal-cli", []Project{{Alias: "ttal", Path: ""}}, ""},
		{"none", "ttal-cli", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchProjectPathByContains(tt.input, tt.projects); got != tt.want {
				t.Errorf("%q=%q want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveProjectAlias_PathMatch(t *testing.T) {
	const a = "proj"
	t.Run("exact", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "c")
		stubProjects(t, []Project{{Alias: a, Path: d}})
		if got := ResolveProjectAlias(d); got != a {
			t.Errorf("%q != %q", got, a)
		}
	})
	t.Run("nested", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "c")
		stubProjects(t, []Project{{Alias: a, Path: d}})
		if got := ResolveProjectAlias(filepath.Join(d, "b")); got != a {
			t.Errorf("%q != %q", got, a)
		}
	})
}

func TestResolveProjectAlias_WorktreePaths(t *testing.T) {
	const a = "proj"
	const r = "~/.ttal/worktrees"
	cases := []struct {
		name, dir string
		stub      string
		want      string
	}{
		{"uuid8-alias", "abc12345-" + a, a, a},
		{"subdir", "deadbeef-" + a + "/src", a, a},
		{"hyphens", "12345678-proj-pr", "proj-pr", "proj-pr"},
		{"unknown", "abc12345-unknown", a, ""},
		{"short uuid", "abc-" + a, a, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stubProjects(t, []Project{{Alias: c.stub, Path: "/p"}})
			if got := resolveProjectAliasInner(filepath.Join(r, c.dir)); got != c.want {
				t.Errorf("%q != %q", got, c.want)
			}
		})
	}
}

func TestResolveProjectAlias_Fallback(t *testing.T) {
	stubProjects(t, []Project{{Alias: "p", Path: "/o"}})
	if got := ResolveProjectAlias(filepath.Join(t.TempDir(), "u")); got != "" {
		t.Errorf("%q", got)
	}
}

func TestResolveGitHubToken(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "t")
		if got := ResolveGitHubToken(""); got != "t" {
			t.Errorf("%q", got)
		}
	})
	t.Run("per-project", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "d")
		t.Setenv("MY", "p")
		stubProjects(t, []Project{{Alias: "x", Path: pathTtal, GitHubTokenEnv: "MY"}})
		if got := ResolveGitHubToken("x"); got != "p" {
			t.Errorf("%q", got)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "d")
		stubProjects(t, []Project{{Alias: "x", Path: pathTtal, GitHubTokenEnv: "M"}})
		if got := ResolveGitHubToken("x"); got != "d" {
			t.Errorf("%q", got)
		}
	})
}
