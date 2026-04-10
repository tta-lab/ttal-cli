package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tta-lab/ttal-cli/internal/skill"
)

func writeSkillFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDisk_ListSkills_TwoFiles(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "breathe", `---
name: breathe
description: Refresh context window
---
# Breathe skill content here.`)
	writeSkillFile(t, dir, "sp-debugging", `---
name: sp-debugging
description: Diagnose bugs systematically
---
# Debug skill body.`)

	skills, err := skill.ListSkills(dir)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	// Check at least one has correct description
	found := map[string]bool{}
	for _, s := range skills {
		found[s.Name] = true
	}
	if !found["breathe"] || !found["sp-debugging"] {
		t.Errorf("expected breathe and sp-debugging, got %v", found)
	}
}

func TestDisk_GetSkill_FrontmatterStripped(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "test-skill", `---
name: test-skill
description: A test skill
---
# Test Skill Body

This is the actual content.`)

	s, err := skill.GetSkill(dir, "test-skill")
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	if s.Name != "test-skill" {
		t.Errorf("expected name test-skill, got %q", s.Name)
	}
	if s.Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", s.Description)
	}
	// Body should have content without frontmatter
	if s.Content == "" {
		t.Error("expected non-empty content")
	}
	assert.Equal(t, "# Test Skill Body\n\nThis is the actual content.", s.Content)
}

func TestDisk_GetSkill_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := skill.GetSkill(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for missing skill")
	}
}

func TestDisk_ListSkills_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	skills, err := skill.ListSkills(dir)
	if err != nil {
		t.Fatalf("ListSkills on empty dir failed: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills in empty dir, got %d", len(skills))
	}
}

func TestDisk_ListSkills_MalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	// File without frontmatter — should be treated as skill with empty name/description
	writeSkillFile(t, dir, "plain-skill", `# Plain Skill

Just body content, no frontmatter.`)

	skills, err := skill.ListSkills(dir)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	// Name comes from filename when frontmatter absent
	if skills[0].Name != "plain-skill" {
		t.Errorf("expected name plain-skill, got %q", skills[0].Name)
	}
	if skills[0].Description != "" {
		t.Errorf("expected empty description, got %q", skills[0].Description)
	}
	// Content should be full file content
	if skills[0].Content == "" {
		t.Error("expected non-empty content for malformed frontmatter file")
	}
}

func TestDisk_GetSkill_NonExistentDir(t *testing.T) {
	_, err := skill.GetSkill("/nonexistent/path", "test")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestFetchContent_DiskFallback_MissingFile(t *testing.T) {
	// FetchContent must return empty string (soft-fail) when file not found.
	// We can't easily override DefaultSkillsDir, so just verify it doesn't panic.
	content := skill.FetchContent("this-skill-does-not-exist")
	if content != "" {
		t.Errorf("expected empty string for missing skill, got %q", content)
	}
}
