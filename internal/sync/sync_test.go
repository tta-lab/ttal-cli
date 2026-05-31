package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployManagerAgents(t *testing.T) {
	teamPath := t.TempDir()

	// Two valid agent dirs
	for _, name := range []string{"yuki", "sage"} {
		os.MkdirAll(filepath.Join(teamPath, name), 0o755) //nolint:errcheck
		os.WriteFile(filepath.Join(teamPath, name, "AGENTS.md"),
			[]byte("---\nname: "+name+"\n---\n# "+name), 0o644) //nolint:errcheck
	}

	// Top-level CLAUDE.user.md — must be ignored (not a dir)
	os.WriteFile(filepath.Join(teamPath, "CLAUDE.user.md"), []byte("# Global"), 0o644) //nolint:errcheck

	// Top-level README.md — must be ignored (not a dir)
	os.WriteFile(filepath.Join(teamPath, "README.md"), []byte("# Readme"), 0o644) //nolint:errcheck

	// Dir with no AGENTS.md — must be skipped without error
	os.MkdirAll(filepath.Join(teamPath, "noidentity"), 0o755) //nolint:errcheck

	// Dot-prefixed dir — must be skipped
	os.MkdirAll(filepath.Join(teamPath, ".hidden"), 0o755)                                   //nolint:errcheck
	os.WriteFile(filepath.Join(teamPath, ".hidden", "AGENTS.md"), []byte("# hidden"), 0o644) //nolint:errcheck

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployManagerAgents(teamPath, false)
	if err != nil {
		t.Fatalf("DeployManagerAgents: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}

	// Verify dest files exist
	for _, name := range []string{"yuki", "sage"} {
		dest := filepath.Join(tmpHome, ".claude", "agents", name+".md")
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("dest file missing: %s", dest)
		}
	}
}

func TestDeployManagerAgentsEmptyTeamPath(t *testing.T) {
	results, err := DeployManagerAgents("", false)
	if err != nil {
		t.Fatalf("DeployManagerAgents with empty path: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestDeployManagerAgentsDryRun(t *testing.T) {
	teamPath := t.TempDir()
	os.MkdirAll(filepath.Join(teamPath, "yuki"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(teamPath, "yuki", "AGENTS.md"),
		[]byte("---\nname: yuki\n---\n# Yuki"), 0o644) //nolint:errcheck

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployManagerAgents(teamPath, true)
	if err != nil {
		t.Fatalf("DeployManagerAgents dry run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Dest should NOT be written in dry run
	dest := filepath.Join(tmpHome, ".claude", "agents", "yuki.md")
	if _, err := os.Stat(dest); err == nil {
		t.Error("dest file should not exist in dry run")
	}
}

func TestDeployManagerAgentsParseError(t *testing.T) {
	teamPath := t.TempDir()
	os.MkdirAll(filepath.Join(teamPath, "bad"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(teamPath, "bad", "AGENTS.md"),
		[]byte("not valid frontmatter\n\n# No dashes"), 0o644) //nolint:errcheck

	_, err := DeployManagerAgents(teamPath, false)
	if err == nil {
		t.Fatal("expected error for malformed AGENTS.md, got nil")
	}
}
