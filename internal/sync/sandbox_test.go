package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// tmuxSockExpected returns the expected tmux socket path for the current process.
func tmuxSockExpected() string {
	return tmuxSocketPath()
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
allowWrite = ["/tmp", "/tmp/worker"]
denyRead = ["~/"]
allowRead = ["."]
`)
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	result, err := syncSandbox(true, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	// Dry-run: settings.json should not exist.
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("dry-run should not write settings.json")
	}

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
}

func TestSyncSandbox_DenyRead(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = []
denyRead = ["~/"]
allowRead = ["~/.ssh", "~/.config/ttal"]
permissionsDeny = [
  "Read(~/.config/ttal/.env)",
  "Read(~/.ssh/id_ed25519)",
]
`)
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	result, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	// denyRead should be ["~/"] expanded.
	home, _ := os.UserHomeDir()
	found := make(map[string]bool)
	for _, p := range result.DenyReadPaths {
		found[p] = true
	}
	if !found[home+"/"] {
		t.Errorf("expected ~/ (home) in denyRead, got %v", result.DenyReadPaths)
	}

	// ~/.ssh should NOT be in denyRead — it's in allowRead.
	if found[filepath.Join(home, ".ssh")] {
		t.Errorf("~/.ssh should be in allowRead, not denyRead")
	}
}

func TestSyncSandbox_AllowReadInFilesystem(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = []
denyRead = ["~/"]
allowRead = ["~/.ssh", "~/.config/ttal", "."]
`)
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), ".claude", "settings.json")
	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	sandbox := settings["sandbox"].(map[string]interface{})
	fs := sandbox["filesystem"].(map[string]interface{})
	allowRead, ok := fs["allowRead"].([]interface{})
	if !ok {
		t.Fatal("expected allowRead in filesystem section")
	}

	home, _ := os.UserHomeDir()
	wantPaths := map[string]bool{
		filepath.Join(home, ".ssh"):         false,
		filepath.Join(home, ".config/ttal"): false,
		".":                                 false,
	}
	for _, v := range allowRead {
		if s, ok := v.(string); ok {
			wantPaths[s] = true
		}
	}
	for p, found := range wantPaths {
		if !found {
			t.Errorf("expected %q in allowRead, got %v", p, allowRead)
		}
	}
}

func TestSyncSandbox_PermissionsDenyWritten(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = []
denyRead = ["~/"]
allowRead = ["~/.ssh", "~/.config/ttal"]
permissionsDeny = [
  "Read(~/.config/ttal/.env)",
  "Read(~/.ssh/id_ed25519)",
]
`)
	writeProjectsConfig(t, dir)

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	_ = json.Unmarshal(data, &settings)

	perms := settings["permissions"].(map[string]interface{})
	deny := perms["deny"].([]interface{})

	home, _ := os.UserHomeDir()
	want := map[string]bool{
		"Read(" + filepath.Join(home, ".config/ttal/.env") + ")": false,
		"Read(" + filepath.Join(home, ".ssh/id_ed25519") + ")":   false,
	}
	for _, v := range deny {
		if s, ok := v.(string); ok {
			want[s] = true
		}
	}
	for entry, found := range want {
		if !found {
			t.Errorf("expected %q in permissions.deny, got %v", entry, deny)
		}
	}
}

func TestSyncSandbox_WritesSettingsJSON(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = ["/tmp"]
denyRead = ["~/"]
allowRead = ["."]
permissionsDeny = ["Read(~/.config/ttal/.env)"]
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

	if _, ok := settings["sandbox"]; !ok {
		t.Error("settings.json missing 'sandbox' key")
	}

	perms, ok := settings["permissions"].(map[string]interface{})
	if !ok {
		t.Fatal("settings.json missing 'permissions' object")
	}
	deny, ok := perms["deny"].([]interface{})
	if !ok {
		t.Fatal("settings.json missing 'permissions.deny' array")
	}
	if len(deny) == 0 {
		t.Error("expected Read() entries in permissions.deny")
	}
}

func TestSyncSandbox_PreservesExistingDenyEntries(t *testing.T) {
	dir := writeSandboxConfig(t, "enabled = true\nallowWrite = []\ndenyRead = []\nallowRead = []")
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
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{home + "/"}, nil, nil, nil, nil)

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
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, nil, []string{extra})

	net := section["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})

	assertSocketPresent(t, sockets, daemonSockExpanded(t))
	assertSocketPresent(t, sockets, extra)
}

func TestBuildSandboxSection_TmuxSocketIncluded(t *testing.T) {
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, nil, nil)

	net := section["network"].(map[string]interface{})
	sockets := net["allowUnixSockets"].([]interface{})

	assertSocketPresent(t, sockets, daemonSockExpanded(t))
	assertSocketPresent(t, sockets, tmuxSockExpected())
}

func TestBuildSandboxSection_AllowedDomains(t *testing.T) {
	domains := []string{"github.com", "*.github.com", "*.guion.io"}
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, domains, nil, nil)

	net, ok := section["network"].(map[string]interface{})
	if !ok {
		t.Fatal("expected network section in sandbox")
	}
	gotDomains, ok := net["allowedDomains"].([]interface{})
	if !ok {
		t.Fatal("expected allowedDomains in network section")
	}
	if len(gotDomains) != len(domains) {
		t.Errorf("expected %d domains, got %d: %v", len(domains), len(gotDomains), gotDomains)
	}
	for i, d := range domains {
		if gotDomains[i] != d {
			t.Errorf("expected domain[%d]=%q, got %q", i, d, gotDomains[i])
		}
	}
}

func TestBuildSandboxSection_NoAllowedDomains(t *testing.T) {
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, nil, nil)
	net := section["network"].(map[string]interface{})
	if _, ok := net["allowedDomains"]; ok {
		t.Error("expected allowedDomains absent when not configured")
	}
}

func TestBuildSandboxSection_AllowReadIncluded(t *testing.T) {
	allowRead := []string{"/home/user/.ssh", "/home/user/.config/ttal"}
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{"/home/user/"}, allowRead, nil, nil, nil)

	fs, ok := section["filesystem"].(map[string]interface{})
	if !ok {
		t.Fatal("expected filesystem section")
	}
	ar, ok := fs["allowRead"].([]interface{})
	if !ok {
		t.Fatal("expected allowRead in filesystem section")
	}
	if len(ar) != 2 {
		t.Errorf("expected 2 allowRead entries, got %d", len(ar))
	}
}

func TestBuildSandboxSection_EmptyAllowReadOmitted(t *testing.T) {
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, nil, nil)
	fs := section["filesystem"].(map[string]interface{})
	if _, ok := fs["allowRead"]; ok {
		t.Error("expected allowRead absent when empty")
	}
}

func TestSyncSandbox_Disabled(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = false
allowWrite = ["~/.ttal"]
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	result, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AllowWritePaths) != 0 {
		t.Errorf("expected empty AllowWritePaths when disabled, got %v", result.AllowWritePaths)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("settings.json should not be written when sandbox is disabled")
	}
}

func TestSyncSandbox_EnforcementFields(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = ["~/.ttal"]
denyRead = ["~/"]
allowRead = ["."]
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
allowWrite = []
denyRead = []
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

// TestSyncSandbox_NonExistentPathsIncluded asserts that non-existent paths still appear
// in allowWrite — settings.json is declarative config, not filtered by what's on disk.
func TestSyncSandbox_NonExistentPathsIncluded(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = ["/nonexistent/path/that/doesnt/exist"]
denyRead = []
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
allowWrite = []
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
allowWrite = []
denyRead = []
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

// TestBuildSandboxSection_DenyWrite verifies that denyWrite is written to the filesystem
// section when non-empty, and omitted when empty.
func TestBuildSandboxSection_DenyWrite(t *testing.T) {
	denyWrite := []string{"/secret/dir", "/readonly/path"}
	section := buildSandboxSection([]string{"/tmp"}, denyWrite, []string{}, nil, nil, nil, nil)

	fs, ok := section["filesystem"].(map[string]interface{})
	if !ok {
		t.Fatal("expected filesystem section")
	}
	dw, ok := fs["denyWrite"].([]interface{})
	if !ok {
		t.Fatal("expected denyWrite in filesystem section")
	}
	if len(dw) != 2 {
		t.Errorf("expected 2 denyWrite entries, got %d: %v", len(dw), dw)
	}
}

// TestBuildSandboxSection_EmptyDenyWriteOmitted verifies that denyWrite is absent when empty.
func TestBuildSandboxSection_EmptyDenyWriteOmitted(t *testing.T) {
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, nil, nil)
	fs := section["filesystem"].(map[string]interface{})
	if _, ok := fs["denyWrite"]; ok {
		t.Error("expected denyWrite absent when empty")
	}
}

// TestBuildSandboxSection_AutoAllowBashSet verifies that autoAllowBashIfSandboxed is written
// when explicitly set via a non-nil pointer.
func TestBuildSandboxSection_AutoAllowBashSet(t *testing.T) {
	val := false
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, &val, nil)
	got, ok := section["autoAllowBashIfSandboxed"].(bool)
	if !ok {
		t.Fatal("expected autoAllowBashIfSandboxed as bool in sandbox section")
	}
	if got != false {
		t.Errorf("expected autoAllowBashIfSandboxed=false, got %v", got)
	}
}

// TestBuildSandboxSection_AutoAllowBashOmittedWhenNil verifies that autoAllowBashIfSandboxed
// is absent from the sandbox section when not set (nil pointer).
func TestBuildSandboxSection_AutoAllowBashOmittedWhenNil(t *testing.T) {
	section := buildSandboxSection([]string{"/tmp"}, nil, []string{}, nil, nil, nil, nil)
	if _, ok := section["autoAllowBashIfSandboxed"]; ok {
		t.Error("expected autoAllowBashIfSandboxed absent when nil")
	}
}

// TestSyncSandbox_DenyWriteWrittenToSettings verifies that denyWrite from sandbox.toml
// appears in settings.json filesystem section.
func TestSyncSandbox_DenyWriteWrittenToSettings(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = ["/tmp"]
denyWrite = ["/tmp/protected"]
denyRead = []
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	sandbox := settings["sandbox"].(map[string]interface{})
	fs := sandbox["filesystem"].(map[string]interface{})
	denyWrite, ok := fs["denyWrite"].([]interface{})
	if !ok {
		t.Fatal("expected denyWrite in filesystem section of settings.json")
	}
	if len(denyWrite) == 0 || denyWrite[0] != "/tmp/protected" {
		t.Errorf("expected /tmp/protected in denyWrite, got %v", denyWrite)
	}
}

// TestSyncSandbox_AutoAllowBashWrittenToSettings verifies that autoAllowBashIfSandboxed
// from sandbox.toml is written to settings.json sandbox section.
func TestSyncSandbox_AutoAllowBashWrittenToSettings(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
autoAllowBashIfSandboxed = false
allowWrite = []
denyRead = []
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	sandbox := settings["sandbox"].(map[string]interface{})
	val, ok := sandbox["autoAllowBashIfSandboxed"].(bool)
	if !ok {
		t.Fatal("expected autoAllowBashIfSandboxed in sandbox section of settings.json")
	}
	if val != false {
		t.Errorf("expected autoAllowBashIfSandboxed=false, got %v", val)
	}
}

// TestSyncSandbox_AllowedDomainsWrittenToSettings verifies that allowedDomains from
// sandbox.toml appear in settings.json network section.
func TestSyncSandbox_AllowedDomainsWrittenToSettings(t *testing.T) {
	dir := writeSandboxConfig(t, `
enabled = true
allowWrite = []
denyRead = []

[network]
allowedDomains = ["github.com", "*.github.com", "*.guion.io"]
`)
	writeProjectsConfig(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")

	_, err := syncSandbox(false, settingsPath)
	if err != nil {
		t.Fatalf("syncSandbox: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	sandbox := settings["sandbox"].(map[string]interface{})
	net := sandbox["network"].(map[string]interface{})
	domains, ok := net["allowedDomains"].([]interface{})
	if !ok {
		t.Fatal("expected allowedDomains in network section of settings.json")
	}

	want := map[string]bool{"github.com": true, "*.github.com": true, "*.guion.io": true}
	for _, d := range domains {
		ds, ok := d.(string)
		if !ok {
			t.Errorf("unexpected non-string domain: %v", d)
			continue
		}
		delete(want, ds)
	}
	if len(want) > 0 {
		t.Errorf("missing domains in allowedDomains: %v", want)
	}
}
