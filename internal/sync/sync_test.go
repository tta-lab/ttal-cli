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

opencode:
  model: anthropic/claude-haiku-4-5-20251001
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

	// Verify OC file was written
	ocContent, err := os.ReadFile(filepath.Join(tmpHome, ".config", "opencode", "agents", "test-bot.md"))
	if err != nil {
		t.Fatalf("reading OC output: %v", err)
	}
	if len(ocContent) == 0 {
		t.Error("OC file is empty")
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

func TestDeploySkills(t *testing.T) {
	srcDir := t.TempDir()

	// Create a skill directory with SKILL.md
	skillDir := filepath.Join(srcDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a non-skill directory (no SKILL.md)
	nonSkillDir := filepath.Join(srcDir, "not-a-skill")
	if err := os.MkdirAll(nonSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeploySkills([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("DeploySkills: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Name != "my-skill" {
		t.Errorf("Name = %q, want %q", results[0].Name, "my-skill")
	}

	// Verify CC symlink exists
	ccDest := filepath.Join(tmpHome, ".claude", "skills", "my-skill")
	info, err := os.Lstat(ccDest)
	if err != nil {
		t.Fatalf("CC symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected CC symlink, got regular file/dir")
	}
	target, err := os.Readlink(ccDest)
	if err != nil {
		t.Fatalf("reading CC symlink: %v", err)
	}
	if target != skillDir {
		t.Errorf("CC symlink target = %q, want %q", target, skillDir)
	}

	// Verify Codex symlink exists
	codexDest := filepath.Join(tmpHome, ".codex", "skills", "my-skill")
	info, err = os.Lstat(codexDest)
	if err != nil {
		t.Fatalf("Codex symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected Codex symlink, got regular file/dir")
	}
	codexTarget, err := os.Readlink(codexDest)
	if err != nil {
		t.Fatalf("reading Codex symlink: %v", err)
	}
	if codexTarget != skillDir {
		t.Errorf("Codex symlink target = %q, want %q", codexTarget, skillDir)
	}
}

func TestDeploySkillsReplacesExistingSymlink(t *testing.T) {
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	destDir := filepath.Join(tmpHome, ".claude", "skills")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create an existing stale symlink
	if err := os.Symlink("/old/path", filepath.Join(destDir, "my-skill")); err != nil {
		t.Fatal(err)
	}

	results, err := DeploySkills([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("DeploySkills: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify CC symlink was replaced
	target, err := os.Readlink(filepath.Join(destDir, "my-skill"))
	if err != nil {
		t.Fatalf("reading CC symlink: %v", err)
	}
	if target != skillDir {
		t.Errorf("CC symlink target = %q, want %q", target, skillDir)
	}

	// Verify Codex symlink was also created
	codexDest := filepath.Join(tmpHome, ".codex", "skills", "my-skill")
	codexTarget, err := os.Readlink(codexDest)
	if err != nil {
		t.Fatalf("reading Codex symlink: %v", err)
	}
	if codexTarget != skillDir {
		t.Errorf("Codex symlink target = %q, want %q", codexTarget, skillDir)
	}
}

func TestCleanSkills(t *testing.T) {
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "keep-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Set up both CC and Codex skill directories
	ccDir := filepath.Join(tmpHome, ".claude", "skills")
	codexDir := filepath.Join(tmpHome, ".codex", "skills")
	for _, d := range []string{ccDir, codexDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Deploy current skill to both
	if err := os.Symlink(skillDir, filepath.Join(ccDir, "keep-skill")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skillDir, filepath.Join(codexDir, "keep-skill")); err != nil {
		t.Fatal(err)
	}

	// Create stale symlinks in both
	if err := os.Symlink("/gone/path", filepath.Join(ccDir, "old-skill")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/gone/path", filepath.Join(codexDir, "old-skill")); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanSkills([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("CleanSkills: %v", err)
	}

	if len(removed) != 2 {
		t.Fatalf("expected 2 removed (CC + Codex), got %d: %v", len(removed), removed)
	}

	// Verify stale symlinks were removed from both
	if _, err := os.Lstat(filepath.Join(ccDir, "old-skill")); !os.IsNotExist(err) {
		t.Error("CC stale symlink should have been removed")
	}
	if _, err := os.Lstat(filepath.Join(codexDir, "old-skill")); !os.IsNotExist(err) {
		t.Error("Codex stale symlink should have been removed")
	}

	// Verify good symlinks still exist in both
	if _, err := os.Lstat(filepath.Join(ccDir, "keep-skill")); err != nil {
		t.Error("CC valid symlink should still exist")
	}
	if _, err := os.Lstat(filepath.Join(codexDir, "keep-skill")); err != nil {
		t.Error("Codex valid symlink should still exist")
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

func TestCleanAgentsOnlyRemovesManaged(t *testing.T) {
	srcDir := t.TempDir()
	agentContent := "---\nname: managed-bot\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(srcDir, "managed-bot.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	ccDir := filepath.Join(tmpHome, ".claude", "agents")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Deploy managed-bot (will have marker)
	if _, err := DeployAgents([]string{srcDir}, false); err != nil {
		t.Fatal(err)
	}

	// Create a stale ttal-managed file
	staleManaged := filepath.Join(ccDir, "old-bot.md")
	if err := os.WriteFile(staleManaged, []byte("---\nname: old-bot\n"+ManagedMarkerField+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a user-created file (no marker)
	userFile := filepath.Join(ccDir, "my-custom-agent.md")
	if err := os.WriteFile(userFile, []byte("---\nname: my-custom-agent\n---\nmy custom agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanAgents([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("CleanAgents: %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d: %v", len(removed), removed)
	}

	// Stale managed file should be removed
	if _, err := os.Stat(staleManaged); !os.IsNotExist(err) {
		t.Error("stale managed file should have been removed")
	}

	// User-created file should still exist
	if _, err := os.Stat(userFile); err != nil {
		t.Error("user-created file should NOT have been removed")
	}

	// Active managed-bot should still exist
	if _, err := os.Stat(filepath.Join(ccDir, "managed-bot.md")); err != nil {
		t.Error("active managed-bot should still exist")
	}
}
