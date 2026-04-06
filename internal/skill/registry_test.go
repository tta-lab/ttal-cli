package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/skill"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	skillBreathe   = "breathe"
	skillDebugging = "sp-debugging"
)

const sampleTOML = `
[skills.breathe]
id = "a1b2c3d4"
category = "command"
description = "Refresh context window"

[skills.sp-debugging]
id = "e5f6a7b8"
category = "methodology"
description = "Debug systematically"

[agents]
yuki = ["breathe", "sp-debugging"]
inke = ["breathe"]
`

func TestLoad_Success(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, err := skill.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	s, err := r.Get(skillBreathe)
	if err != nil {
		t.Fatalf("Get breathe failed: %v", err)
	}
	if s.FlicknoteID != "a1b2c3d4" {
		t.Errorf("expected id a1b2c3d4, got %s", s.FlicknoteID)
	}
	if s.Category != "command" {
		t.Errorf("expected category command, got %s", s.Category)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")
	r, err := skill.Load(path)
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if len(r.List()) != 0 {
		t.Error("expected empty list")
	}
}

func TestGet_NotFound(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown skill")
	}
}

func TestList_Sorted(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(list))
	}
	if list[0].Name != skillBreathe || list[1].Name != skillDebugging {
		t.Errorf("expected sorted order, got %v, %v", list[0].Name, list[1].Name)
	}
}

func TestListForAgent_Filtered(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	list := r.ListForAgent("inke")
	if len(list) != 1 || list[0].Name != skillBreathe {
		t.Errorf("expected [breathe] for inke, got %v", list)
	}
}

func TestListForAgent_UnknownAgent_ReturnsAll(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	list := r.ListForAgent("unknown-agent")
	if len(list) != 2 {
		t.Errorf("expected all 2 skills for unknown agent, got %d", len(list))
	}
}

func TestAdd_PersistsToFile(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	err := r.Add(skill.Skill{
		Name:        "new-skill",
		FlicknoteID: "c9d0e1f2",
		Category:    "reference",
		Description: "A new skill",
	}, false)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Reload and verify
	r2, _ := skill.Load(path)
	s, err := r2.Get("new-skill")
	if err != nil {
		t.Fatalf("skill not persisted: %v", err)
	}
	if s.FlicknoteID != "c9d0e1f2" {
		t.Errorf("expected c9d0e1f2, got %s", s.FlicknoteID)
	}
}

func TestAdd_ForceOverwrites(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	err := r.Add(skill.Skill{
		Name:        skillBreathe,
		FlicknoteID: "newid123",
		Category:    "command",
		Description: "Updated",
	}, true)
	if err != nil {
		t.Fatalf("Add with force failed: %v", err)
	}

	s, _ := r.Get(skillBreathe)
	if s.FlicknoteID != "newid123" {
		t.Errorf("expected newid123, got %s", s.FlicknoteID)
	}
}

func TestAdd_ErrorIfExistsNoForce(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	err := r.Add(skill.Skill{Name: skillBreathe, FlicknoteID: "xx"}, false)
	if err == nil {
		t.Error("expected error when adding existing skill without force")
	}
}

func TestRemove_PersistsToFile(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	removed, agents, err := r.Remove(skillBreathe)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if removed.FlicknoteID != "a1b2c3d4" {
		t.Errorf("expected returned skill id a1b2c3d4, got %s", removed.FlicknoteID)
	}
	if len(agents) != 2 {
		t.Errorf("expected removed from 2 agents (inke, yuki), got %v", agents)
	}

	// Reload and verify
	r2, _ := skill.Load(path)
	_, err = r2.Get(skillBreathe)
	if err == nil {
		t.Error("skill should be removed")
	}
}

func TestRemove_CleansAgentAllowLists(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	_, agents, _ := r.Remove(skillBreathe)

	// Both yuki and inke had breathe
	found := map[string]bool{}
	for _, a := range agents {
		found[a] = true
	}
	if !found["yuki"] || !found["inke"] {
		t.Errorf("expected inke and yuki in cleaned agents, got %v", agents)
	}
}

func TestValidate_DanglingReference(t *testing.T) {
	toml := `
[skills.breathe]
id = "a1b2c3d4"
category = "command"
description = "Refresh"

[agents]
yuki = ["breathe", "nonexistent-skill"]
`
	path := writeTOML(t, toml)
	r, err := skill.Load(path)
	if err != nil {
		// warnings are printed but not fatal
		t.Fatalf("Load failed: %v", err)
	}
	warnings := r.Validate()
	if len(warnings) == 0 {
		t.Error("expected warning for dangling reference")
	}
}

func TestReverseLookup(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	s, ok := r.ReverseLookup("a1b2c3d4")
	if !ok {
		t.Fatal("expected to find skill by flicknote ID")
	}
	if s.Name != skillBreathe {
		t.Errorf("expected breathe, got %s", s.Name)
	}

	_, ok = r.ReverseLookup("xxxxxxxx")
	if ok {
		t.Error("expected not found for unknown ID")
	}
}

func TestParseFrontmatter_WithFrontmatter(t *testing.T) {
	content := []byte(`---
name: my-skill
description: Does something useful
---
# My Skill

This is the body content.
`)
	name, desc, body := skill.ParseFrontmatter(content)
	if name != "my-skill" {
		t.Errorf("expected name my-skill, got %q", name)
	}
	if desc != "Does something useful" {
		t.Errorf("expected description, got %q", desc)
	}
	if string(body) != "# My Skill\n\nThis is the body content.\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte("# Just body\n\nNo frontmatter here.\n")
	name, desc, body := skill.ParseFrontmatter(content)
	if name != "" || desc != "" {
		t.Errorf("expected empty name/desc, got %q/%q", name, desc)
	}
	if string(body) != string(content) {
		t.Error("body should equal full content when no frontmatter")
	}
}

func TestParseFrontmatter_CRLF(t *testing.T) {
	content := []byte("---\r\nname: my-skill\r\ndescription: Useful thing\r\n---\r\n# Body\r\n")
	name, desc, body := skill.ParseFrontmatter(content)
	if name != "my-skill" {
		t.Errorf("expected name my-skill, got %q", name)
	}
	if desc != "Useful thing" {
		t.Errorf("expected description Useful thing, got %q", desc)
	}
	if string(body) != "# Body\r\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestParseFrontmatter_NoTrailingNewline(t *testing.T) {
	// Frontmatter with no trailing newline after closing ---
	content := []byte("---\nname: skill\n---")
	_, _, body := skill.ParseFrontmatter(content)
	// Body should be empty (not a panic or partial content)
	if len(body) != 0 {
		t.Errorf("expected empty body, got %q", string(body))
	}
}

func TestParseFrontmatter_Unterminated(t *testing.T) {
	content := []byte("---\nname: skill\nno closing delimiter\n")
	name, desc, body := skill.ParseFrontmatter(content)
	// Unterminated frontmatter returns full content unchanged
	if name != "" || desc != "" {
		t.Errorf("expected empty name/desc for unterminated, got %q/%q", name, desc)
	}
	if string(body) != string(content) {
		t.Error("body should equal full content for unterminated frontmatter")
	}
}

func TestRemove_NotFound(t *testing.T) {
	path := writeTOML(t, sampleTOML)
	r, _ := skill.Load(path)

	_, _, err := r.Remove("nonexistent")
	if err == nil {
		t.Error("expected error when removing nonexistent skill")
	}
}

func TestReverseLookup_PrefixMatch(t *testing.T) {
	// Skills with IDs longer than 8 chars should still match on 8-char prefix
	tomlContent := `
[skills.breathe]
id = "a1b2c3d4e5f6"
category = "command"
description = "Refresh"
`
	path := writeTOML(t, tomlContent)
	r, _ := skill.Load(path)

	// Full ID match
	s, ok := r.ReverseLookup("a1b2c3d4e5f6")
	if !ok || s.Name != skillBreathe {
		t.Errorf("expected breathe on full ID match, got %v, %v", s, ok)
	}

	// Prefix match
	s, ok = r.ReverseLookup("a1b2c3d4")
	if !ok || s.Name != skillBreathe {
		t.Errorf("expected breathe on prefix match, got %v, %v", s, ok)
	}
}

func TestFetchContent_FakeFlicknote(t *testing.T) {
	// Create a temp dir with a fake flicknote binary
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "flicknote")
	if err := os.WriteFile(fakeBin, []byte(`#!/bin/sh
if [ "$1" = "content" ] && [ "$2" = "a1b2c3d4" ]; then
	echo "# Breathe skill body"
elif [ "$1" = "content" ] && [ "$2" = "e5f6a7b8" ]; then
	echo "# SpDebugging content"
else
	echo "unknown" >&2
	exit 1
fi
`), 0o755); err != nil {
		t.Fatal(err)
	}

	// Build new PATH with tmpDir first
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	_ = os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)

	// Patch DefaultPath to use a temp registry
	origDefaultPath := skill.DefaultPath
	registryPath := writeTOML(t, sampleTOML)
	skill.DefaultPath = func() string { return registryPath }
	t.Cleanup(func() { skill.DefaultPath = origDefaultPath })

	content := skill.FetchContent("breathe")
	if content == "" {
		t.Fatal("FetchContent returned empty string")
	}
	if content != "# Breathe skill body" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestFetchContent_NotFound(t *testing.T) {
	// Patch DefaultPath to use a temp registry with no matching skill
	origDefaultPath := skill.DefaultPath
	registryPath := writeTOML(t, sampleTOML)
	skill.DefaultPath = func() string { return registryPath }
	t.Cleanup(func() { skill.DefaultPath = origDefaultPath })

	// Even without a fake flicknote, FetchContent should not panic on skill-not-found
	content := skill.FetchContent("nonexistent-skill")
	if content != "" {
		t.Errorf("expected empty for nonexistent skill, got: %q", content)
	}
}

func TestFetchContents_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "flicknote")
	if err := os.WriteFile(fakeBin, []byte(`#!/bin/sh
if [ "$1" = "content" ] && [ "$2" = "a1b2c3d4" ]; then
	echo "breathe body"
elif [ "$1" = "content" ] && [ "$2" = "e5f6a7b8" ]; then
	echo "debugging body"
else
	echo "unknown" >&2
	exit 1
fi
`), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	_ = os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)

	origDefaultPath := skill.DefaultPath
	registryPath := writeTOML(t, sampleTOML)
	skill.DefaultPath = func() string { return registryPath }
	t.Cleanup(func() { skill.DefaultPath = origDefaultPath })

	content := skill.FetchContents([]string{"breathe", "sp-debugging"})
	if content == "" {
		t.Fatal("FetchContents returned empty string")
	}
	if !strings.Contains(content, "# breathe [skill]") {
		t.Errorf("missing breathe header: %q", content)
	}
	if !strings.Contains(content, "# sp-debugging [skill]") {
		t.Errorf("missing sp-debugging header: %q", content)
	}
	if !strings.Contains(content, "breathe body") {
		t.Errorf("missing breathe body: %q", content)
	}
	if !strings.Contains(content, "debugging body") {
		t.Errorf("missing debugging body: %q", content)
	}
}

func TestFetchContents_EmptyNames(t *testing.T) {
	content := skill.FetchContents([]string{})
	if content != "" {
		t.Errorf("expected empty for empty names, got: %q", content)
	}
}
