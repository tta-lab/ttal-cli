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
enabled = true

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
	dir := writeSandboxConfig(t, "enabled = true\n[shared]\nextra_paths = []")
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
enabled = true

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
	dir := writeSandboxConfig(t, "enabled = true\n[shared]\nextra_paths = []")
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

func TestBuildSandboxSection_EnforcementFields(t *testing.T) {
	home, _ := os.UserHomeDir()
	section := buildSandboxSection([]string{"/tmp"}, []string{home + "/.config/ttal/.env"}, nil)

	if section["enabled"] != true {
		t.Error("expected enabled=true")
	}
	if section["failIfUnavailable"] != true {
		t.Error("expected failIfUnavailable=true")
	}
	if section["allowUnsandboxedCommands"] != false {
		t.Error("expected allowUnsandboxedCommands=false")
	}

	// Assert daemon socket is present and expanded.
	net, ok := section["network"].(map[string]interface{})
	if !ok {
		t.Fatal("expected network section in sandbox")
	}
	sockets, ok := net["allowUnixSockets"].([]interface{})
	if !ok {
		t.Fatal("expected allowUnixSockets in network")
	}
	daemonSock := home + "/.ttal/daemon.sock"
	found := false
	for _, s := range sockets {
		if s == daemonSock {
			found = true
		}
	}
	if !found {
		t.Errorf("expected daemon socket %s in sockets, got %v", daemonSock, sockets)
	}
}

func TestBuildSandboxSection_PreservesExistingSockets(t *testing.T) {
	home, _ := os.UserHomeDir()
	extra := "/run/user/1000/custom.sock"
	section := buildSandboxSection([]string{"/tmp"}, []string{}, []string{extra})

	net := section["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})

	// Daemon socket and user socket both present, no duplicates.
	daemonSock := home + "/.ttal/daemon.sock"
	foundDaemon, foundExtra := false, false
	for _, s := range sockets {
		switch s {
		case daemonSock:
			foundDaemon = true
		case extra:
			foundExtra = true
		}
	}
	if !foundDaemon {
		t.Errorf("expected daemon socket in sockets, got %v", sockets)
	}
	if !foundExtra {
		t.Errorf("expected existing socket %s preserved, got %v", extra, sockets)
	}
}

func TestSyncSandbox_Disabled(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = false

[shared]
extra_paths = ["~/.ttal:rw"]
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	result, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Result should be empty — sandbox sync skipped entirely.
	if len(result.AllowWritePaths) != 0 {
		t.Errorf("expected empty AllowWritePaths when disabled, got %v", result.AllowWritePaths)
	}
	// settings.json must not be written.
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("settings.json should not be written when sandbox is disabled")
	}
}

func TestSyncSandbox_EnforcementFields(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true

[shared]
extra_paths = ["~/.ttal:rw"]
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	sandbox, ok := settings["sandbox"].(map[string]interface{})
	if !ok {
		t.Fatal("missing sandbox section in settings.json")
	}

	if sandbox["enabled"] != true {
		t.Error("expected enabled=true")
	}
	if sandbox["failIfUnavailable"] != true {
		t.Error("expected failIfUnavailable=true")
	}
	if sandbox["allowUnsandboxedCommands"] != false {
		t.Error("expected allowUnsandboxedCommands=false")
	}

	// Daemon socket must be present.
	home, _ := os.UserHomeDir()
	net := sandbox["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})
	expected := home + "/.ttal/daemon.sock"
	found := false
	for _, s := range sockets {
		if s == expected {
			found = true
		}
	}
	if !found {
		t.Errorf("expected daemon socket %s in sockets, got %v", expected, sockets)
	}
}

func TestSyncSandbox_PreservesExistingSockets(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true

[shared]
extra_paths = []
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	// Pre-populate settings.json with a custom socket.
	customSock := "/run/user/1000/custom.sock"
	initial := `{"sandbox": {"network": {"allowUnixSockets": ["` + customSock + `"]}}}`
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
	sandbox := settings["sandbox"].(map[string]interface{})
	net := sandbox["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})

	home, _ := os.UserHomeDir()
	daemonSock := home + "/.ttal/daemon.sock"
	foundDaemon, foundCustom := false, false
	for _, s := range sockets {
		switch s {
		case daemonSock:
			foundDaemon = true
		case customSock:
			foundCustom = true
		}
	}
	if !foundDaemon {
		t.Errorf("expected daemon socket in output sockets, got %v", sockets)
	}
	if !foundCustom {
		t.Errorf("expected custom socket %s preserved, got %v", customSock, sockets)
	}
}

func TestSyncSandbox_NonExistentPathsIncluded(t *testing.T) {
	// Non-existent paths should appear in allowWrite — settings.json is declarative.
	dir := writeSandboxConfig(t, `
enabled = true

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
