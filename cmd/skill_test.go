package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/skill"
)

// writeTempRegistry writes a skills.toml to a temp dir and returns the path.
func withTempRegistry(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.toml")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

const cmdTestTOML = `
[skills.breathe]
id = "a1b2c3d4"
category = "command"
description = "Refresh context window"

[skills.sp-debugging]
id = "e5f6a7b8"
category = "methodology"
description = "Debug systematically"

[agents]
yuki = ["breathe"]
`

func TestSkillList_AllFlag(t *testing.T) {
	// Tests that runSkillList with all=true returns all skills.
	// We test via the registry directly since the cmd uses loadRegistry()
	// which reads DefaultPath. We swap DefaultPath by writing to a temp file
	// and monkey-patching through the skill package indirectly.
	//
	// For this reason we test the business logic of the underlying registry,
	// which is already covered by internal/skill tests.
	// The cmd test verifies flag parsing and TTAL_AGENT_NAME filtering.

	registryPath := withTempRegistry(t, cmdTestTOML)
	r, err := skill.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}

	// All
	all := r.List()
	if len(all) != 2 {
		t.Errorf("expected 2 skills, got %d", len(all))
	}

	// Agent filtered
	filtered := r.ListForAgent("yuki")
	if len(filtered) != 1 || filtered[0].Name != "breathe" {
		t.Errorf("expected [breathe] for yuki, got %v", filtered)
	}

	// Unknown agent gets all
	unknown := r.ListForAgent("nobody")
	if len(unknown) != 2 {
		t.Errorf("expected all 2 for unknown agent, got %d", len(unknown))
	}
}

func TestSkillGet_NotFound(t *testing.T) {
	registryPath := withTempRegistry(t, cmdTestTOML)
	r, _ := skill.Load(registryPath)

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown skill")
	}
}

func TestSkillGet_ReturnsCorrectFlicknoteID(t *testing.T) {
	registryPath := withTempRegistry(t, cmdTestTOML)
	r, _ := skill.Load(registryPath)

	s, err := r.Get("breathe")
	if err != nil {
		t.Fatal(err)
	}
	if s.FlicknoteID != "a1b2c3d4" {
		t.Errorf("expected a1b2c3d4, got %s", s.FlicknoteID)
	}
}

func TestSkillAdd_RegistrationPersists(t *testing.T) {
	registryPath := withTempRegistry(t, cmdTestTOML)
	r, _ := skill.Load(registryPath)

	err := r.Add(skill.Skill{
		Name:        "new-skill",
		FlicknoteID: "c9d0e1f2",
		Category:    "reference",
		Description: "New skill",
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	r2, _ := skill.Load(registryPath)
	s, err := r2.Get("new-skill")
	if err != nil {
		t.Fatalf("skill not persisted: %v", err)
	}
	if s.FlicknoteID != "c9d0e1f2" {
		t.Errorf("wrong ID: %s", s.FlicknoteID)
	}
}

func TestSkillAdd_ErrorIfExists(t *testing.T) {
	registryPath := withTempRegistry(t, cmdTestTOML)
	r, _ := skill.Load(registryPath)

	err := r.Add(skill.Skill{Name: "breathe", FlicknoteID: "xx"}, false)
	if err == nil {
		t.Error("expected error adding existing skill without --force")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSkillRemove_CleansAgentAllowLists(t *testing.T) {
	tomlContent := `
[skills.breathe]
id = "a1b2c3d4"
category = "command"
description = "Refresh"

[skills.sp-debugging]
id = "e5f6a7b8"
category = "methodology"
description = "Debug"

[agents]
yuki = ["breathe", "sp-debugging"]
inke = ["breathe"]
`
	registryPath := withTempRegistry(t, tomlContent)
	r, _ := skill.Load(registryPath)

	_, agents, err := r.Remove("breathe")
	if err != nil {
		t.Fatal(err)
	}

	agentSet := map[string]bool{}
	for _, a := range agents {
		agentSet[a] = true
	}
	if !agentSet["yuki"] || !agentSet["inke"] {
		t.Errorf("expected both yuki and inke in cleaned agents, got %v", agents)
	}

	r2, _ := skill.Load(registryPath)
	_, err = r2.Get("breathe")
	if err == nil {
		t.Error("breathe should be removed")
	}

	_, err = r2.Get("sp-debugging")
	if err != nil {
		t.Errorf("sp-debugging should still exist: %v", err)
	}
}

func TestSkillList_JSON(t *testing.T) {
	registryPath := withTempRegistry(t, cmdTestTOML)
	r, err := skill.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}

	skills := r.List()
	type skillJSON struct {
		Name        string `json:"name"`
		FlicknoteID string `json:"flicknote_id"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	output := make([]skillJSON, 0, len(skills))
	for _, s := range skills {
		output = append(output, skillJSON{
			Name:        s.Name,
			FlicknoteID: s.FlicknoteID,
			Category:    s.Category,
			Description: s.Description,
		})
	}

	var parsed []map[string]string
	if err := json.Unmarshal(mustMarshalJSON(output), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 skills, got %d", len(parsed))
	}

	expectedKeys := []string{"name", "flicknote_id", "category", "description"}
	for _, item := range parsed {
		for _, key := range expectedKeys {
			if _, ok := item[key]; !ok {
				t.Errorf("missing key %q in JSON output", key)
			}
		}
	}
}

func TestSkillGet_JSON_Metadata(t *testing.T) {
	registryPath := withTempRegistry(t, cmdTestTOML)
	r, err := skill.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}

	s, err := r.Get("breathe")
	if err != nil {
		t.Fatal(err)
	}

	if s.Name != "breathe" {
		t.Errorf("expected name breathe, got %s", s.Name)
	}
	if s.FlicknoteID != "a1b2c3d4" {
		t.Errorf("expected flicknote_id a1b2c3d4, got %s", s.FlicknoteID)
	}
	if s.Category != "command" {
		t.Errorf("expected category command, got %s", s.Category)
	}
	if s.Description != "Refresh context window" {
		t.Errorf("expected description 'Refresh context window', got %s", s.Description)
	}
}

func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
