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

	// Verify CC directory was copied
	ccDest := filepath.Join(tmpHome, ".claude", "skills", "my-skill")
	info, err := os.Lstat(ccDest)
	if err != nil {
		t.Fatalf("CC skill dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected CC skill to be a directory")
	}
	if _, err := os.ReadFile(filepath.Join(ccDest, "SKILL.md")); err != nil {
		t.Errorf("CC SKILL.md not copied: %v", err)
	}

	// Verify Codex directory was copied
	codexDest := filepath.Join(tmpHome, ".codex", "skills", "my-skill")
	info, err = os.Lstat(codexDest)
	if err != nil {
		t.Fatalf("Codex skill dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected Codex skill to be a directory")
	}
	if _, err := os.ReadFile(filepath.Join(codexDest, "SKILL.md")); err != nil {
		t.Errorf("Codex SKILL.md not copied: %v", err)
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

	// Verify CC directory was created (replacing old symlink)
	ccDest := filepath.Join(destDir, "my-skill")
	info, err := os.Lstat(ccDest)
	if err != nil {
		t.Fatalf("CC skill dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected CC skill to be a real directory, not a symlink")
	}
	if _, err := os.ReadFile(filepath.Join(ccDest, "SKILL.md")); err != nil {
		t.Errorf("CC SKILL.md not copied: %v", err)
	}

	// Verify Codex directory was also created
	codexDest := filepath.Join(tmpHome, ".codex", "skills", "my-skill")
	codexInfo, err := os.Lstat(codexDest)
	if err != nil {
		t.Fatalf("Codex skill dir not created: %v", err)
	}
	if !codexInfo.IsDir() {
		t.Error("expected Codex skill to be a real directory")
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

	// Deploy current skill to both (as real directories)
	for _, dir := range []string{filepath.Join(ccDir, "keep-skill"), filepath.Join(codexDir, "keep-skill")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create stale directories in both
	for _, dir := range []string{filepath.Join(ccDir, "old-skill"), filepath.Join(codexDir, "old-skill")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := CleanSkills([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("CleanSkills: %v", err)
	}

	if len(removed) != 2 {
		t.Fatalf("expected 2 removed (CC + Codex), got %d: %v", len(removed), removed)
	}

	// Verify stale directories were removed from both
	if _, err := os.Lstat(filepath.Join(ccDir, "old-skill")); !os.IsNotExist(err) {
		t.Error("CC stale skill dir should have been removed")
	}
	if _, err := os.Lstat(filepath.Join(codexDir, "old-skill")); !os.IsNotExist(err) {
		t.Error("Codex stale skill dir should have been removed")
	}

	// Verify valid skill dirs still exist in both
	if _, err := os.Lstat(filepath.Join(ccDir, "keep-skill")); err != nil {
		t.Error("CC valid skill dir should still exist")
	}
	if _, err := os.Lstat(filepath.Join(codexDir, "keep-skill")); err != nil {
		t.Error("Codex valid skill dir should still exist")
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

func TestDeploySkillsRecursive(t *testing.T) {
	srcDir := t.TempDir()

	// Create a skill with nested subdirectory structure
	skillDir := filepath.Join(srcDir, "my-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "templates", "example.md"), []byte("# Example"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	if _, err := DeploySkills([]string{srcDir}, false); err != nil {
		t.Fatalf("DeploySkills: %v", err)
	}

	ccDest := filepath.Join(tmpHome, ".claude", "skills", "my-skill")
	// Verify nested file was copied
	nestedPath := filepath.Join(ccDest, "templates", "example.md")
	data, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("nested file not copied: %v", err)
	}
	if string(data) != "# Example" {
		t.Errorf("nested file content = %q, want %q", string(data), "# Example")
	}
}

func TestDeploySkillsReplacesExistingDirectory(t *testing.T) {
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# New"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-populate dest with a stale real directory (simulates a previous sync)
	ccDest := filepath.Join(tmpHome, ".claude", "skills", "my-skill")
	if err := os.MkdirAll(ccDest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ccDest, "OLD.md"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := DeploySkills([]string{srcDir}, false); err != nil {
		t.Fatalf("DeploySkills: %v", err)
	}

	// Verify stale file was replaced
	if _, err := os.Stat(filepath.Join(ccDest, "OLD.md")); !os.IsNotExist(err) {
		t.Error("stale OLD.md should have been replaced by fresh copy")
	}
	if _, err := os.ReadFile(filepath.Join(ccDest, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not present after re-sync: %v", err)
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
