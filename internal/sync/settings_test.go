package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDenyPrimaryAgentsAsSubagents_FreshFile(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	added, err := denyPrimaryAgentsAsSubagents([]string{"athena", "kestrel"}, false, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(added) != 2 {
		t.Fatalf("expected 2 added, got %d: %v", len(added), added)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	deny, _ := perms["deny"].([]interface{})

	if len(deny) != 2 {
		t.Fatalf("expected 2 deny entries, got %d", len(deny))
	}
	if deny[0] != "Agent(athena)" {
		t.Errorf("deny[0] = %q, want %q", deny[0], "Agent(athena)")
	}
	if deny[1] != "Agent(kestrel)" {
		t.Errorf("deny[1] = %q, want %q", deny[1], "Agent(kestrel)")
	}
}

func TestDenyPrimaryAgentsAsSubagents_PreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	initial := map[string]interface{}{
		"permissions": map[string]interface{}{
			"deny": []interface{}{
				"EnterPlanMode",
				"Agent(claude-code-guide)",
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := denyPrimaryAgentsAsSubagents([]string{"athena", "kestrel"}, false, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(added) != 2 {
		t.Fatalf("expected 2 added, got %d: %v", len(added), added)
	}

	written, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(written, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	deny, _ := perms["deny"].([]interface{})

	if len(deny) != 4 {
		t.Fatalf("expected 4 deny entries, got %d: %v", len(deny), deny)
	}

	// Existing entries must still be first (order preserved)
	if deny[0] != "EnterPlanMode" {
		t.Errorf("deny[0] = %q, want %q", deny[0], "EnterPlanMode")
	}
	if deny[1] != "Agent(claude-code-guide)" {
		t.Errorf("deny[1] = %q, want %q", deny[1], "Agent(claude-code-guide)")
	}
	// New entries appended at end
	if deny[2] != "Agent(athena)" {
		t.Errorf("deny[2] = %q, want %q", deny[2], "Agent(athena)")
	}
	if deny[3] != "Agent(kestrel)" {
		t.Errorf("deny[3] = %q, want %q", deny[3], "Agent(kestrel)")
	}
}

func TestDenyPrimaryAgentsAsSubagents_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	agents := []string{"athena", "kestrel"}

	// First run
	if _, err := denyPrimaryAgentsAsSubagents(agents, false, settingsPath); err != nil {
		t.Fatalf("first run error: %v", err)
	}

	// Second run — should add nothing
	added, err := denyPrimaryAgentsAsSubagents(agents, false, settingsPath)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("expected 0 added on second run, got %d: %v", len(added), added)
	}

	// Verify deny list has exactly 2 entries (no duplicates)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	perms, _ := settings["permissions"].(map[string]interface{})
	deny, _ := perms["deny"].([]interface{})

	if len(deny) != 2 {
		t.Errorf("expected 2 deny entries after idempotent run, got %d", len(deny))
	}
}

func TestDenyPrimaryAgentsAsSubagents_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	added, err := denyPrimaryAgentsAsSubagents([]string{"athena"}, true, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(added) != 1 || added[0] != "athena" {
		t.Errorf("expected [athena] added, got %v", added)
	}

	// File must NOT have been created in dry-run mode
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("dry-run should not create settings.json")
	}
}

func TestDenyPrimaryAgentsAsSubagents_DryRunExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	initial := map[string]interface{}{
		"permissions": map[string]interface{}{
			"deny": []interface{}{"EnterPlanMode"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	originalContent := append(data, '\n')
	if err := os.WriteFile(settingsPath, originalContent, 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := denyPrimaryAgentsAsSubagents([]string{"athena"}, true, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(added) != 1 || added[0] != "athena" {
		t.Errorf("expected [athena] added, got %v", added)
	}

	// File content must be unchanged
	written, _ := os.ReadFile(settingsPath)
	if string(written) != string(originalContent) {
		t.Error("dry-run should not modify existing settings.json")
	}
}

func TestDenyPrimaryAgentsAsSubagents_NonArrayDenyReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Write settings.json where permissions.deny is null (not an array)
	content := []byte(`{"permissions": {"deny": null}}` + "\n")
	if err := os.WriteFile(settingsPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := denyPrimaryAgentsAsSubagents([]string{"athena"}, false, settingsPath)
	if err == nil {
		t.Fatal("expected error for non-array deny, got nil")
	}

	// File must not have been modified
	written, err2 := os.ReadFile(settingsPath)
	if err2 != nil {
		t.Fatalf("reading settings.json after error: %v", err2)
	}
	if string(written) != string(content) {
		t.Error("settings.json should not be modified when returning an error")
	}
}

func TestDenyPrimaryAgentsAsSubagents_NonObjectPermissionsReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Write settings.json where permissions is a string (not an object)
	content := []byte(`{"permissions": "invalid"}` + "\n")
	if err := os.WriteFile(settingsPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := denyPrimaryAgentsAsSubagents([]string{"athena"}, false, settingsPath)
	if err == nil {
		t.Fatal("expected error for non-object permissions, got nil")
	}

	// File must not have been modified
	written, err2 := os.ReadFile(settingsPath)
	if err2 != nil {
		t.Fatalf("reading settings.json after error: %v", err2)
	}
	if string(written) != string(content) {
		t.Error("settings.json should not be modified when returning an error")
	}
}
