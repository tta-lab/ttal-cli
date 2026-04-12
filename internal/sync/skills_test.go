package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeploySkills_DirBasedLayout(t *testing.T) {
	// Test that dir-based skills (name/SKILL.md) are deployed correctly
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create a dir-based skill: skills/sp-planning/SKILL.md
	skillDir := filepath.Join(srcDir, "sp-planning")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: sp-planning\ndescription: Full planning process\n---\n# Planning\nBody here."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := DeploySkills([]string{srcDir}, destDir, false)
	if err != nil {
		t.Fatalf("DeploySkills failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	want := filepath.Join(destDir, "sp-planning", "SKILL.md")
	if results[0].Dest != want {
		t.Errorf("expected dest %q, got %q", want, results[0].Dest)
	}

	// Verify file was actually copied
	if _, err := os.Stat(want); err != nil {
		t.Errorf("skill file not copied to dest: %v", err)
	}

	// Verify content is preserved
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != skillContent {
		t.Errorf("content mismatch:\nwant: %q\ngot: %q", skillContent, string(data))
	}
}

func TestDeploySkills_FlatFileLayout(t *testing.T) {
	// Test that flat skill files (name.md) are deployed correctly
	srcDir := t.TempDir()
	destDir := t.TempDir()

	skillContent := "---\nname: breathe\ndescription: Refresh context\n---\n# Breathe\nBody here."
	if err := os.WriteFile(filepath.Join(srcDir, "breathe.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := DeploySkills([]string{srcDir}, destDir, false)
	if err != nil {
		t.Fatalf("DeploySkills failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	want := filepath.Join(destDir, "breathe", "SKILL.md")
	if results[0].Dest != want {
		t.Errorf("expected dest %q, got %q", want, results[0].Dest)
	}
}

func TestDeploySkills_DryRun(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	skillContent := "---\nname: breathe\ndescription: Refresh\n---\n# Breathe"
	if err := os.WriteFile(filepath.Join(srcDir, "breathe.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := DeploySkills([]string{srcDir}, destDir, true)
	if err != nil {
		t.Fatalf("DeploySkills dry run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify file was NOT copied (dry run)
	want := filepath.Join(destDir, "breathe", "SKILL.md")
	if _, err := os.Stat(want); err == nil {
		t.Error("dry run should not copy files")
	}
}

func TestDeploySkills_NonExistentSourcePath_ReturnsError(t *testing.T) {
	_, err := DeploySkills([]string{"/nonexistent/path"}, "/tmp/dest", false)
	if err == nil {
		t.Error("expected error for nonexistent source path")
	}
}

func TestDeploySkills_MultipleSkills(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create multiple skills
	files := map[string]string{
		"breathe.md":      "---\nname: breathe\n---\n# Breathe",
		"sp-planning.md":  "---\nname: sp-planning\n---\n# Planning",
		"sp-debugging.md": "---\nname: sp-debugging\n---\n# Debug",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := DeploySkills([]string{srcDir}, destDir, false)
	if err != nil {
		t.Fatalf("DeploySkills failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestDeploySkills_CreatesDestDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "nested", "path")

	skillContent := "---\nname: breathe\n---\n# Breathe"
	if err := os.WriteFile(filepath.Join(srcDir, "breathe.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := DeploySkills([]string{srcDir}, destDir, false)
	if err != nil {
		t.Fatalf("DeploySkills failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify dest dir was created
	if info, err := os.Stat(destDir); err != nil || !info.IsDir() {
		t.Error("dest dir was not created")
	}
}
