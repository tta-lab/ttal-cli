package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// ── SessionStart hook tests ─────────────────────────────────────────────────

func TestInstallSessionStartHook_FreshFile(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	added, err := installSessionStartHook(false, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected added=true for fresh install")
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	if !strings.Contains(string(data), "ttal context") {
		t.Errorf("expected 'ttal context' in settings.json, got: %s", data)
	}
	if !strings.Contains(string(data), "SessionStart") {
		t.Errorf("expected 'SessionStart' key in settings.json, got: %s", data)
	}

	// Verify hook is written under the "hooks" wrapper key (CC 2.1.87+ format).
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected settings.hooks to be an object, got: %T", settings["hooks"])
	}
	entries, ok := hooksMap["SessionStart"].([]interface{})
	if !ok {
		t.Fatalf("expected hooks.SessionStart to be an array, got: %T", hooksMap["SessionStart"])
	}
	// Verify the matcher value — a regression to "*" would silently fire on resume/compact.
	found := false
	for _, e := range entries {
		m, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if m["matcher"] == "startup|clear" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected matcher 'startup|clear' in written hook entry, got: %s", data)
	}
}

func TestInstallSessionStartHook_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// First install
	if _, err := installSessionStartHook(false, settingsPath); err != nil {
		t.Fatalf("first install error: %v", err)
	}

	// Second install — should not add duplicate
	added, err := installSessionStartHook(false, settingsPath)
	if err != nil {
		t.Fatalf("second install error: %v", err)
	}
	if added {
		t.Error("expected added=false on second (idempotent) install")
	}

	// Verify only one ttal context entry exists
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	count := strings.Count(string(data), "ttal context")
	if count != 1 {
		t.Errorf("expected exactly 1 'ttal context' entry, found %d", count)
	}
}

func TestInstallSessionStartHook_PreservesExistingHooks(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Write a settings.json with an existing non-ttal SessionStart hook (new hooks wrapper format).
	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"SessionStart": []interface{}{
				map[string]interface{}{
					"matcher": "startup|clear",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "python-linter",
							"timeout": 10,
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := installSessionStartHook(false, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected added=true when adding to existing SessionStart")
	}

	written, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	// Both the existing hook and the new ttal hook must be present.
	if !strings.Contains(string(written), "python-linter") {
		t.Error("existing non-ttal hook was not preserved")
	}
	if !strings.Contains(string(written), "ttal context") {
		t.Error("new ttal context hook not added")
	}
}

func TestInstallSessionStartHook_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	added, err := installSessionStartHook(true, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected added=true in dry-run (reports what would be done)")
	}

	// File must NOT have been created in dry-run mode.
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("dry-run should not create settings.json")
	}
}

func TestInstallSessionStartHook_MigratesLegacyTopLevelSessionStart(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Write pre-2.1.87 settings.json with SessionStart at the top level.
	initial := map[string]interface{}{
		"SessionStart": []interface{}{
			map[string]interface{}{
				"matcher": "*",
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": "legacy-hook",
						"timeout": 10,
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := installSessionStartHook(false, settingsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected added=true after migrating legacy format")
	}

	written, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(written, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Old top-level key must be gone.
	if _, ok := settings["SessionStart"]; ok {
		t.Error("legacy top-level SessionStart key should have been removed after migration")
	}

	// Both the migrated legacy hook and the new ttal hook must be under hooks.SessionStart.
	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected hooks wrapper object, got: %T", settings["hooks"])
	}
	entries, ok := hooksMap["SessionStart"].([]interface{})
	if !ok {
		t.Fatalf("expected hooks.SessionStart array, got: %T", hooksMap["SessionStart"])
	}
	if !strings.Contains(string(written), "legacy-hook") {
		t.Error("migrated legacy hook should be present in hooks.SessionStart")
	}
	if !strings.Contains(string(written), "ttal context") {
		t.Error("new ttal context hook should be present in hooks.SessionStart")
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries after migration (legacy + ttal), got %d", len(entries))
	}
}

func TestInstallSessionStartHook_NonObjectHooksReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	content := []byte(`{"hooks": "not-an-object"}` + "\n")
	if err := os.WriteFile(settingsPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := installSessionStartHook(false, settingsPath)
	if err == nil {
		t.Fatal("expected error for non-object hooks, got nil")
	}

	// File must not have been modified.
	written, _ := os.ReadFile(settingsPath)
	if string(written) != string(content) {
		t.Error("settings.json should not be modified when returning an error")
	}
}

func TestInstallSessionStartHook_NonArraySessionStartReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	content := []byte(`{"hooks": {"SessionStart": "not-an-array"}}` + "\n")
	if err := os.WriteFile(settingsPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := installSessionStartHook(false, settingsPath)
	if err == nil {
		t.Fatal("expected error for non-array hooks.SessionStart, got nil")
	}

	// File must not have been modified.
	written, _ := os.ReadFile(settingsPath)
	if string(written) != string(content) {
		t.Error("settings.json should not be modified when returning an error")
	}
}
