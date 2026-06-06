package daemon

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
)

func TestGatherProjectPaths(t *testing.T) {
	orig := gatherProjectPathsListFn
	t.Cleanup(func() { gatherProjectPathsListFn = orig })

	gatherProjectPathsListFn = func() ([]project.Project, error) {
		return []project.Project{
			{Alias: "alpha", Name: "Alpha", Path: "/proj/alpha"},
			{Alias: "beta", Name: "Beta", Path: "/proj/beta"},
		}, nil
	}

	cfg := &config.Config{}
	paths := gatherProjectPaths(cfg)

	want := []string{"/proj/alpha", "/proj/beta"}
	if len(paths) != len(want) {
		t.Fatalf("expected %d paths, got %d: %v", len(want), len(paths), paths)
	}
	for i, p := range want {
		if paths[i] != p {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], p)
		}
	}
}

func TestGatherProjectPaths_EmptyStore(t *testing.T) {
	orig := gatherProjectPathsListFn
	t.Cleanup(func() { gatherProjectPathsListFn = orig })

	gatherProjectPathsListFn = func() ([]project.Project, error) {
		return nil, nil
	}

	cfg := &config.Config{}
	paths := gatherProjectPaths(cfg)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for empty store, got %v", paths)
	}
}

func TestGatherProjectPaths_NoProjects(t *testing.T) {
	orig := gatherProjectPathsListFn
	t.Cleanup(func() { gatherProjectPathsListFn = orig })

	gatherProjectPathsListFn = func() ([]project.Project, error) {
		return nil, nil
	}

	cfg := &config.Config{}
	paths := gatherProjectPaths(cfg)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths with no teams, got %v", paths)
	}
}

func TestBuildManagerAgentEnv(t *testing.T) {
	t.Run("includes identity and 1h prompt cache flag", func(t *testing.T) {
		cfg := &config.Config{}
		vars := buildManagerAgentEnv("yuki", cfg)
		joined := strings.Join(vars, "\n")

		if !strings.Contains(joined, "TTAL_AGENT_NAME=yuki") {
			t.Errorf("TTAL_AGENT_NAME missing from %v", vars)
		}
		if !strings.Contains(joined, "ENABLE_PROMPT_CACHING_1H=1") {
			t.Errorf("ENABLE_PROMPT_CACHING_1H=1 missing from %v — 1h TTL opt-in is required for manager sessions", vars)
		}
	})
}
