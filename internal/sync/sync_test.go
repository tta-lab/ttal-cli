package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployGlobalPrompt(t *testing.T) {
	srcFile := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(srcFile, []byte("# Global Prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployGlobalPrompt(srcFile, false)
	if err != nil {
		t.Fatalf("DeployGlobalPrompt: %v", err)
	}

	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}

	// Verify CC file was written as a real file
	ccDest := filepath.Join(tmpHome, ".claude", "CLAUDE.md")
	data, err := os.ReadFile(ccDest)
	if err != nil {
		t.Fatalf("CC CLAUDE.md not created: %v", err)
	}
	if string(data) != "# Global Prompt" {
		t.Errorf("CC content = %q, want %q", string(data), "# Global Prompt")
	}
	info, _ := os.Lstat(ccDest)
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("CC CLAUDE.md should be a real file, not a symlink")
	}
}

func TestDeployGlobalPromptReplacesExistingSymlink(t *testing.T) {
	srcFile := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(srcFile, []byte("# Updated"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-create CC dir with an existing symlink at dest
	ccDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ccDest := filepath.Join(ccDir, "CLAUDE.md")
	if err := os.Symlink("/old/path", ccDest); err != nil {
		t.Fatal(err)
	}

	if _, err := DeployGlobalPrompt(srcFile, false); err != nil {
		t.Fatalf("DeployGlobalPrompt: %v", err)
	}

	data, err := os.ReadFile(ccDest)
	if err != nil {
		t.Fatalf("CC CLAUDE.md not readable after replacing symlink: %v", err)
	}
	if string(data) != "# Updated" {
		t.Errorf("content = %q, want %q", string(data), "# Updated")
	}
	info, _ := os.Lstat(ccDest)
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("CC CLAUDE.md should be a real file, not a symlink")
	}
}

func TestDeployGlobalPromptReplacesExistingFile(t *testing.T) {
	srcFile := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(srcFile, []byte("# New Content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-create an existing regular file at dest
	ccDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ccDest := filepath.Join(ccDir, "CLAUDE.md")
	if err := os.WriteFile(ccDest, []byte("# Old Content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := DeployGlobalPrompt(srcFile, false); err != nil {
		t.Fatalf("DeployGlobalPrompt: %v", err)
	}

	data, err := os.ReadFile(ccDest)
	if err != nil {
		t.Fatalf("CC CLAUDE.md not readable: %v", err)
	}
	if string(data) != "# New Content" {
		t.Errorf("content = %q, want %q", string(data), "# New Content")
	}
}

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
