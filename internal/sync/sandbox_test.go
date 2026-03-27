package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// daemonSockExpanded returns the expanded daemon socket path for assertions.
func daemonSockExpanded(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return filepath.Join(home, ".ttal", "daemon.sock")
}

// assertSocketPresent fails the test if target is not found in sockets.
func assertSocketPresent(t *testing.T, sockets []interface{}, target string) {
	t.Helper()
	for _, s := range sockets {
		if s == target {
			return
		}
	}
	t.Errorf("expected socket %s in %v", target, sockets)
}

// assertEnforcementFields verifies the hardcoded sandbox enforcement fields are set correctly.
func assertEnforcementFields(t *testing.T, sandbox map[string]interface{}) {
	t.Helper()
	if sandbox["enabled"] != true {
		t.Error("expected enabled=true")
	}
	if sandbox["failIfUnavailable"] != true {
		t.Error("expected failIfUnavailable=true")
	}
	if sandbox["allowUnsandboxedCommands"] != false {
		t.Error("expected allowUnsandboxedCommands=false")
	}
}

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

	assertEnforcementFields(t, section)

	net, ok := section["network"].(map[string]interface{})
	if !ok {
		t.Fatal("expected network section in sandbox")
	}
	sockets, ok := net["allowUnixSockets"].([]interface{})
	if !ok {
		t.Fatal("expected allowUnixSockets in network")
	}
	assertSocketPresent(t, sockets, daemonSockExpanded(t))
}

func TestBuildSandboxSection_PreservesExistingSockets(t *testing.T) {
	extra := "/run/user/1000/custom.sock"
	section := buildSandboxSection([]string{"/tmp"}, []string{}, []string{extra})

	net := section["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})

	assertSocketPresent(t, sockets, daemonSockExpanded(t))
	assertSocketPresent(t, sockets, extra)
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

	assertEnforcementFields(t, sandbox)

	net := sandbox["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})
	assertSocketPresent(t, sockets, daemonSockExpanded(t))
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

	assertSocketPresent(t, sockets, daemonSockExpanded(t))
	assertSocketPresent(t, sockets, customSock)
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

// TestSyncSandbox_MissingConfigFileSkipsEnforcement asserts that when sandbox.toml
// is absent (fresh install), syncSandbox returns empty and does not write settings.json.
func TestSyncSandbox_MissingConfigFileSkipsEnforcement(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Write projects.toml but NOT sandbox.toml.
	ttalDir := filepath.Join(dir, "ttal")
	if err := os.MkdirAll(ttalDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ttalDir, "projects.toml"), nil, 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}

	settingsPath := filepath.Join(dir, "settings.json")
	result, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("expected no error when sandbox.toml absent, got: %v", err)
	}
	if len(result.AllowWritePaths) != 0 {
		t.Errorf("expected empty result with no sandbox.toml, got %v", result.AllowWritePaths)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("settings.json should not be written when sandbox.toml is absent")
	}
}

// TestSyncSandbox_DisabledLeavesExistingSettingsUnchanged verifies that when sandbox
// is disabled and settings.json already contains sandbox enforcement from a previous run,
// the file is left untouched (stale enforcement remains — caller is responsible for cleanup).
func TestSyncSandbox_DisabledLeavesExistingSettingsUnchanged(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = false

[shared]
extra_paths = []
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	// Pre-populate with active enforcement from a previous sync.
	prior := `{"sandbox":{"enabled":true,"failIfUnavailable":true}}`
	if err := os.WriteFile(settingsPath, []byte(prior), 0o644); err != nil {
		t.Fatalf("write prior settings: %v", err)
	}

	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	if string(data) != prior {
		t.Errorf("expected settings.json unchanged when disabled, got: %s", data)
	}
}

// TestSyncSandbox_ParseErrorReturnsError verifies that a corrupt sandbox.toml propagates
// an error rather than silently disabling enforcement (security-relevant).
func TestSyncSandbox_ParseErrorReturnsError(t *testing.T) {
	dir := writeSandboxConfig(t, `this is not valid toml !!!`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	_, err := syncSandbox(false, settingsPath)
	if err == nil {
		t.Error("expected error when sandbox.toml is malformed, got nil")
	}
}

// TestSyncSandbox_DaemonSocketDeduplication verifies that running ttal sync twice
// does not produce duplicate daemon socket entries in network.allowUnixSockets.
func TestSyncSandbox_DaemonSocketDeduplication(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true

[shared]
extra_paths = []
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	// First sync.
	if _, err := syncSandbox(false, settingsPath); err != nil {
		t.Fatalf("first syncSandbox: %v", err)
	}
	// Second sync — daemon socket already present in settings.json.
	if _, err := syncSandbox(false, settingsPath); err != nil {
		t.Fatalf("second syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	sandbox := settings["sandbox"].(map[string]interface{})
	net := sandbox["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})

	daemonSock := daemonSockExpanded(t)
	count := 0
	for _, s := range sockets {
		if s == daemonSock {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected daemon socket exactly once, found %d times in %v", count, sockets)
	}
}
