package daemon

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
	"github.com/tta-lab/ttal-cli/internal/project"
)

func TestGatherProjectPaths(t *testing.T) {
	// Create a temp project store.
	team1Dir := t.TempDir()

	store1 := project.NewStore(filepath.Join(team1Dir, "projects.toml"))
	if err := store1.Add("alpha", "Alpha", "/proj/alpha"); err != nil {
		t.Fatalf("store1.Add alpha: %v", err)
	}
	if err := store1.Add("beta", "Beta", "/proj/beta"); err != nil {
		t.Fatalf("store1.Add beta: %v", err)
	}

	storeMap := map[string]string{
		"default": filepath.Join(team1Dir, "projects.toml"),
	}
	storePathFn := func(teamName string) string { return storeMap[teamName] }

	cfg := &config.Config{}

	paths := gatherProjectPaths(cfg, storePathFn)

	// Expect sorted paths.
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
	tmpDir := t.TempDir()
	// Store exists but has no projects.
	storePathFn := func(_ string) string { return filepath.Join(tmpDir, "projects.toml") }

	cfg := &config.Config{}

	paths := gatherProjectPaths(cfg, storePathFn)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for empty store, got %v", paths)
	}
}

func TestGatherProjectPaths_NoProjects(t *testing.T) {
	cfg := &config.Config{}
	paths := gatherProjectPaths(cfg, func(_ string) string { return "/nonexistent" })
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

	t.Run("includes TTAL_HUMAN when admin human is set", func(t *testing.T) {
		cfg := &config.Config{
			AdminHuman: &humanfs.Human{Alias: "neil", Name: "Neil"},
		}
		vars := buildManagerAgentEnv("yuki", cfg)
		joined := strings.Join(vars, "\n")

		if !strings.Contains(joined, "TTAL_HUMAN=neil") {
			t.Errorf("TTAL_HUMAN=neil missing from %v", vars)
		}
	})

}
