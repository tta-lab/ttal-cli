package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustMkdirAll is a test helper that creates directories or fails the test.
func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
}

// mustWriteFile is a test helper that writes a file or fails the test.
func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestDeployRules_SubdirScan(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Create skill dir with RULE.md
	skillDir := filepath.Join(src, "ttal-cli")
	mustMkdirAll(t, skillDir)
	mustWriteFile(t, filepath.Join(skillDir, "RULE.md"), []byte("# ttal cheat sheet\n"))

	// Create skill dir without RULE.md (should be skipped)
	mustMkdirAll(t, filepath.Join(src, "no-rule"))

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
	mustWriteFile(t, filepath.Join(src, "RULE.md"), []byte("# direct rule\n"))

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
	mustMkdirAll(t, skillDir)
	mustWriteFile(t, filepath.Join(skillDir, "RULE.md"), []byte("content"))

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

func TestDeployCodexRules(t *testing.T) {
	src := t.TempDir()
	agentsPath := filepath.Join(t.TempDir(), "AGENTS.md")

	// Create two rule sources
	for _, name := range []string{"proj-a", "proj-b"} {
		dir := filepath.Join(src, name)
		mustMkdirAll(t, dir)
		mustWriteFile(t, filepath.Join(dir, "RULE.md"), []byte("## "+name+" commands"))
	}

	rules, err := DeployRulesTo([]string{src}, t.TempDir(), true)
	if err != nil {
		t.Fatalf("DeployRulesTo: %v", err)
	}

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
	mustWriteFile(t, agentsPath, []byte("# My Agents\n\nSome existing content.\n"))

	dir := filepath.Join(src, "my-proj")
	mustMkdirAll(t, dir)
	mustWriteFile(t, filepath.Join(dir, "RULE.md"), []byte("hot commands"))

	rules, err := DeployRulesTo([]string{src}, t.TempDir(), true)
	if err != nil {
		t.Fatalf("DeployRulesTo: %v", err)
	}

	// Deploy twice
	if err := DeployCodexRulesTo(rules, agentsPath, false); err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	if err := DeployCodexRulesTo(rules, agentsPath, false); err != nil {
		t.Fatalf("second deploy: %v", err)
	}

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
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
