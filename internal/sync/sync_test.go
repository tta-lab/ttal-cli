package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployAgents(t *testing.T) {
	// Create temp source dir with a canonical agent
	srcDir := t.TempDir()
	agentContent := `---
name: test-bot
description: A test bot

claude-code:
  model: haiku
---

You are a test bot.
`
	if err := os.WriteFile(filepath.Join(srcDir, "test-bot.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME to a temp dir so we don't touch real ~/.claude
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployAgents([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("DeployAgents: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Name != "test-bot" {
		t.Errorf("Name = %q, want %q", r.Name, "test-bot")
	}

	// Verify CC file was written
	ccContent, err := os.ReadFile(filepath.Join(tmpHome, ".claude", "agents", "test-bot.md"))
	if err != nil {
		t.Fatalf("reading CC output: %v", err)
	}
	if len(ccContent) == 0 {
		t.Error("CC file is empty")
	}
}

func TestDeployAgentsDryRun(t *testing.T) {
	srcDir := t.TempDir()
	agentContent := "---\nname: dry-agent\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(srcDir, "dry-agent.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployAgents([]string{srcDir}, true)
	if err != nil {
		t.Fatalf("DeployAgents dry-run: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify nothing was actually written
	if _, err := os.Stat(filepath.Join(tmpHome, ".claude", "agents", "dry-agent.md")); !os.IsNotExist(err) {
		t.Error("dry-run should not create files")
	}
}

func TestDeployAgentsNonexistentPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployAgents([]string{"/nonexistent/path"}, false)
	if err != nil {
		t.Fatalf("should not error on missing path: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

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
