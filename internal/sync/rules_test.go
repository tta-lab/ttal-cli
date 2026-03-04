package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployRules_SubdirScan(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Create skill dir with RULE.md
	skillDir := filepath.Join(src, "ttal-cli")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "RULE.md"), []byte("# ttal cheat sheet\n"), 0o644)

	// Create skill dir without RULE.md (should be skipped)
	os.MkdirAll(filepath.Join(src, "no-rule"), 0o755)

	results, err := DeployRulesTo([]string{src}, dest, false)
	if err != nil {
		t.Fatalf("DeployRulesTo: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "ttal-cli" {
		t.Errorf("expected name ttal-cli, got %s", results[0].Name)
	}

	content, err := os.ReadFile(filepath.Join(dest, "ttal-cli.md"))
	if err != nil {
		t.Fatalf("reading deployed rule: %v", err)
	}
	if string(content) != "# ttal cheat sheet\n" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestDeployRules_DirectRuleMD(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Place RULE.md directly in the path root (simulates ~/Code/ttal-cli with RULE.md at root)
	os.WriteFile(filepath.Join(src, "RULE.md"), []byte("# direct rule\n"), 0o644)

	results, err := DeployRulesTo([]string{src}, dest, false)
	if err != nil {
		t.Fatalf("DeployRulesTo: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != filepath.Base(src) {
		t.Errorf("expected name %s, got %s", filepath.Base(src), results[0].Name)
	}

	content, err := os.ReadFile(results[0].Dest)
	if err != nil {
		t.Fatalf("reading deployed rule: %v", err)
	}
	if string(content) != "# direct rule\n" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestDeployRules_DryRun(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "rules")

	skillDir := filepath.Join(src, "my-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "RULE.md"), []byte("content"), 0o644)

	results, err := DeployRulesTo([]string{src}, dest, true)
	if err != nil {
		t.Fatalf("DeployRulesTo: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Dest directory should not exist in dry run
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("expected dest directory to not exist in dry run")
	}
}

func TestCleanRules(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Create a valid rule
	skillDir := filepath.Join(src, "valid-rule")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "RULE.md"), []byte("valid"), 0o644)

	// Deploy the valid rule
	DeployRulesTo([]string{src}, dest, false)

	// Add a stale rule file manually
	os.WriteFile(filepath.Join(dest, "stale-rule.md"), []byte("stale"), 0o644)

	removed, err := CleanRulesIn([]string{src}, dest, false)
	if err != nil {
		t.Fatalf("CleanRulesIn: %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}
	if !strings.HasSuffix(removed[0], "stale-rule.md") {
		t.Errorf("expected stale-rule.md removed, got %s", removed[0])
	}

	// Verify stale file is gone
	if _, err := os.Stat(filepath.Join(dest, "stale-rule.md")); !os.IsNotExist(err) {
		t.Error("stale rule should have been removed")
	}

	// Verify valid file still exists
	if _, err := os.Stat(filepath.Join(dest, "valid-rule.md")); err != nil {
		t.Error("valid rule should still exist")
	}
}

func TestDeployCodexRules(t *testing.T) {
	src := t.TempDir()
	agentsPath := filepath.Join(t.TempDir(), "AGENTS.md")

	// Create two rule sources
	for _, name := range []string{"proj-a", "proj-b"} {
		dir := filepath.Join(src, name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "RULE.md"), []byte("## "+name+" commands"), 0o644)
	}

	rules, _ := DeployRulesTo([]string{src}, t.TempDir(), true)

	if err := DeployCodexRulesTo(rules, agentsPath, false); err != nil {
		t.Fatalf("DeployCodexRulesTo: %v", err)
	}

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, codexRulesMarkerStart) {
		t.Error("missing start marker")
	}
	if !strings.Contains(s, codexRulesMarkerEnd) {
		t.Error("missing end marker")
	}
	if !strings.Contains(s, "### proj-a") {
		t.Error("missing proj-a section")
	}
	if !strings.Contains(s, "### proj-b") {
		t.Error("missing proj-b section")
	}
}

func TestDeployCodexRules_Idempotent(t *testing.T) {
	src := t.TempDir()
	agentsPath := filepath.Join(t.TempDir(), "AGENTS.md")

	// Pre-existing content
	os.WriteFile(agentsPath, []byte("# My Agents\n\nSome existing content.\n"), 0o644)

	dir := filepath.Join(src, "my-proj")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "RULE.md"), []byte("hot commands"), 0o644)

	rules, _ := DeployRulesTo([]string{src}, t.TempDir(), true)

	// Deploy twice
	DeployCodexRulesTo(rules, agentsPath, false)
	DeployCodexRulesTo(rules, agentsPath, false)

	content, _ := os.ReadFile(agentsPath)
	s := string(content)

	// Should have exactly one managed section
	if strings.Count(s, codexRulesMarkerStart) != 1 {
		t.Errorf("expected 1 start marker, got %d", strings.Count(s, codexRulesMarkerStart))
	}

	// Existing content should be preserved
	if !strings.Contains(s, "# My Agents") {
		t.Error("existing content was lost")
	}
}
