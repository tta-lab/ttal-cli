package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSandboxConfig writes a sandbox.toml in a temp XDG_CONFIG_HOME dir.
func writeSandboxConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	ttalDir := filepath.Join(dir, "ttal")
	if err := os.MkdirAll(ttalDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ttalDir, "sandbox.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write sandbox.toml: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

// writeProjectsConfig writes a projects.toml in the given config dir.
func writeProjectsConfig(t *testing.T, cfgDir string) {
	t.Helper()
	ttalDir := filepath.Join(cfgDir, "ttal")
	if err := os.MkdirAll(ttalDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ttalDir, "projects.toml"), nil, 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}
}

func TestSyncSandbox_DryRun(t *testing.T) {
	dir := writeSandboxConfig(t, `
[shared]
extra_paths = ["/tmp:rw", "/usr/local:ro"]

[worker]
extra_paths = ["/tmp/worker:rw"]

[manager]
extra_paths = []
`)
	writeProjectsConfig(t, dir)

	// Use temp settings.json path.
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	result, err := syncSandbox(true, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	// Dry-run: settings.json should not exist.
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("dry-run should not write settings.json")
	}

	// /tmp:rw and /tmp/worker:rw should be in allowWrite.
	found := make(map[string]bool)
	for _, p := range result.AllowWritePaths {
		found[p] = true
	}
	if !found["/tmp"] {
		t.Errorf("expected /tmp in allowWrite, got %v", result.AllowWritePaths)
	}
	if !found["/tmp/worker"] {
		t.Errorf("expected /tmp/worker in allowWrite, got %v", result.AllowWritePaths)
	}
	// /usr/local:ro should NOT be in allowWrite
	if found["/usr/local"] {
		t.Errorf("expected /usr/local (ro) NOT in allowWrite, but found it")
	}
}

func TestSyncSandbox_DenyRead(t *testing.T) {
	dir := writeSandboxConfig(t, "[shared]\nextra_paths = []")
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	result, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	// denyRead must contain all secret paths.
	found := make(map[string]bool)
	for _, p := range result.DenyReadPaths {
		found[p] = true
	}

	home, _ := os.UserHomeDir()
	expected := []string{
		filepath.Join(home, ".config/ttal/.env"),
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".netrc"),
		filepath.Join(home, ".aws/credentials"),
		filepath.Join(home, ".kube/config"),
	}
	for _, p := range expected {
		if !found[p] {
			t.Errorf("expected %s in denyRead, got %v", p, result.DenyReadPaths)
		}
	}
}

func TestSyncSandbox_WritesSettingsJSON(t *testing.T) {
	dir := writeSandboxConfig(t, `
[shared]
extra_paths = ["/tmp:rw"]
[worker]
extra_paths = []
[manager]
extra_paths = []
`)
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), ".claude", "settings.json")
	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	// Check sandbox section exists.
	if _, ok := settings["sandbox"]; !ok {
		t.Error("settings.json missing 'sandbox' key")
	}

	// Check permissions.deny has Read entries.
	perms, ok := settings["permissions"].(map[string]interface{})
	if !ok {
		t.Fatal("settings.json missing 'permissions' object")
	}
	deny, ok := perms["deny"].([]interface{})
	if !ok {
		t.Fatal("settings.json missing 'permissions.deny' array")
	}
	joined := make([]string, len(deny))
	for i, v := range deny {
		joined[i], _ = v.(string)
	}
	hasReadEntry := false
	for _, s := range joined {
		if strings.HasPrefix(s, "Read(") {
			hasReadEntry = true
			break
		}
	}
	if !hasReadEntry {
		t.Errorf("expected Read() entries in permissions.deny, got %v", joined)
	}
}

func TestSyncSandbox_PreservesExistingDenyEntries(t *testing.T) {
	dir := writeSandboxConfig(t, "[shared]\nextra_paths = []")
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), "settings.json")

	// Pre-populate settings.json with existing deny entries.
	initial := `{"permissions": {"deny": ["Agent(kestrel)", "Agent(mira)"]}}`
	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	_ = json.Unmarshal(data, &settings)

	perms := settings["permissions"].(map[string]interface{})
	deny := perms["deny"].([]interface{})

	// Existing Agent entries must still be present.
	joined := make([]string, len(deny))
	for i, v := range deny {
		joined[i], _ = v.(string)
	}
	for _, name := range []string{"Agent(kestrel)", "Agent(mira)"} {
		found := false
		for _, s := range joined {
			if s == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected existing entry %q preserved in deny list, got %v", name, joined)
		}
	}
}

func TestSyncSandbox_NonExistentPathsIncluded(t *testing.T) {
	// Non-existent paths should appear in allowWrite — settings.json is declarative.
	dir := writeSandboxConfig(t, `
[shared]
extra_paths = ["/nonexistent/path/that/doesnt/exist:rw"]
[worker]
extra_paths = []
[manager]
extra_paths = []
`)
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	result, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	found := false
	for _, p := range result.AllowWritePaths {
		if p == "/nonexistent/path/that/doesnt/exist" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected non-existent path in allowWrite (declarative config), got %v", result.AllowWritePaths)
	}
}
