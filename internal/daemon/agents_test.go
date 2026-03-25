package daemon

import (
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
)

func TestGatherProjectPaths(t *testing.T) {
	// Create two temp project stores simulating two teams.
	team1Dir := t.TempDir()
	team2Dir := t.TempDir()

	store1 := project.NewStore(filepath.Join(team1Dir, "projects.toml"))
	if err := store1.Add("alpha", "Alpha", "/proj/alpha"); err != nil {
		t.Fatalf("store1.Add alpha: %v", err)
	}
	if err := store1.Add("beta", "Beta", "/proj/beta"); err != nil {
		t.Fatalf("store1.Add beta: %v", err)
	}

	store2 := project.NewStore(filepath.Join(team2Dir, "projects.toml"))
	if err := store2.Add("gamma", "Gamma", "/proj/gamma"); err != nil {
		t.Fatalf("store2.Add gamma: %v", err)
	}
	// Duplicate path — should appear only once.
	if err := store2.Add("alpha2", "Alpha Dup", "/proj/alpha"); err != nil {
		t.Fatalf("store2.Add alpha2: %v", err)
	}

	storeMap := map[string]string{
		"team1": filepath.Join(team1Dir, "projects.toml"),
		"team2": filepath.Join(team2Dir, "projects.toml"),
	}
	storePathFn := func(teamName string) string { return storeMap[teamName] }

	mcfg := &config.DaemonConfig{
		Teams: map[string]*config.ResolvedTeam{
			"team1": {},
			"team2": {},
		},
	}

	paths := gatherProjectPaths(mcfg, storePathFn)

	// Expect sorted, deduplicated paths.
	want := []string{"/proj/alpha", "/proj/beta", "/proj/gamma"}
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

	mcfg := &config.DaemonConfig{
		Teams: map[string]*config.ResolvedTeam{
			"default": {},
		},
	}

	paths := gatherProjectPaths(mcfg, storePathFn)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for empty store, got %v", paths)
	}
}

func TestGatherProjectPaths_NoTeams(t *testing.T) {
	mcfg := &config.DaemonConfig{
		Teams: map[string]*config.ResolvedTeam{},
	}
	paths := gatherProjectPaths(mcfg, func(_ string) string { return "/nonexistent" })
	if len(paths) != 0 {
		t.Errorf("expected 0 paths with no teams, got %v", paths)
	}
}
