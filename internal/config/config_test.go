package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSessionName(t *testing.T) {
	tests := []struct {
		team  string
		agent string
		want  string
	}{
		{"default", "kestrel", "ttal-default-kestrel"},
		{"default", "yuki", "ttal-default-yuki"},
		{"guion", "mira", "ttal-default-mira"},
		{"sven", "athena", "ttal-default-athena"},
	}
	for _, tt := range tests {
		if got := AgentSessionName(tt.agent); got != tt.want {
			t.Errorf("AgentSessionName(%q) = %q, want %q", tt.agent, got, tt.want)
		}
	}
}

func TestShellField(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"empty config empty shell", &Config{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Shell; got != tt.want {
				t.Errorf("Config.Shell = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoad_Success(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NEIL_CHAT_ID", "12345")

	cfgDir := filepath.Join(home, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `references_path = "~/code/references"

[teams.default]
team_path = "/tmp/team"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humansContent := `[neil]
name = "Neil"
admin = true
`
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansContent), 0644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.TeamPath != "/tmp/team" {
		t.Errorf("TeamPath = %q, want %q", cfg.TeamPath, "/tmp/team")
	}
	wantRefs := filepath.Join(home, "code", "references")
	if got := cfg.AskReferencesPath(); got != wantRefs {
		t.Errorf("AskReferencesPath() = %q, want %q", got, wantRefs)
	}
}

// TestLoad_HumanChatIDFromEnv verifies chat IDs come from env, not TOML/config.
func TestLoad_HumanChatIDFromEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NEIL_CHAT_ID", "845849177")
	cfgDir := filepath.Join(home, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configContent := `[teams.default]
team_path = "/tmp/team"
chat_id = "legacy-wrong"
`
	humansContent := `[neil]
name = "Neil"
telegram_chat_id = "legacy-wrong"
admin = true
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansContent), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.AdminHuman == nil {
		t.Fatal("cfg.AdminHuman is nil")
	}
	if cfg.AdminHuman.Alias != "neil" { //nolint:goconst // test fixture uses "neil"
		t.Errorf("AdminHuman.Alias = %q, want neil", cfg.AdminHuman.Alias)
	}
	if cfg.AdminHuman.Name != "Neil" { //nolint:goconst // test fixture uses "Neil"
		t.Errorf("AdminHuman.Name = %q, want Neil", cfg.AdminHuman.Name)
	}
	if cfg.AdminHuman.TelegramChatID != "845849177" { //nolint:goconst // test fixture uses "845849177"
		t.Errorf("AdminHuman.TelegramChatID = %q, want 845849177", cfg.AdminHuman.TelegramChatID)
	}
	if !cfg.AdminHuman.Admin {
		t.Error("AdminHuman.Admin = false, want true")
	}
	// Env wins over legacy TOML/config fields.
	if cfg.ChatID != "845849177" { //nolint:goconst // test fixture uses "845849177"
		t.Errorf("cfg.ChatID = %q, want 845849177 (from NEIL_CHAT_ID)", cfg.ChatID)
	}
	if cfg.UserName != "Neil" { //nolint:goconst // test fixture uses "Neil"
		t.Errorf("cfg.UserName = %q, want Neil", cfg.UserName)
	}
}

// TestLoad_HumansAbsent verifies error when humans.toml is absent (no legacy fallback).
func TestLoad_HumansAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configContent := `[teams.default]
team_path = "/tmp/team"
chat_id = "12345"
[user]
name = "Neil"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when humans.toml is absent, got nil")
	}
	if !strings.Contains(err.Error(), "humans.toml") {
		t.Errorf("error = %q, want substring %q", err.Error(), "humans.toml")
	}
}

func TestLoad_MissingTeamsDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "ttal", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `# No [teams.default] section
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load() = nil, want error for missing [teams.default]")
	}
}

func TestLoad_MissingTeamPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "ttal", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `[teams.default]
team_path = ""
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load() = nil, want error for empty team_path")
	}
}

func TestLoad_InvalidRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "ttal", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `[teams.default]
team_path = "/tmp/team"
default_runtime = "invalid_runtime"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load() = nil, want error for invalid runtime")
	}
}

func TestLoad_InvalidMergeMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "ttal", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `[teams.default]
team_path = "/tmp/team"
merge_mode = "invalid"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load() = nil, want error for invalid merge_mode")
	}
}

func TestBuildEnvShellCommand(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		envParts []string
		cmd      string
		want     string
	}{
		{"no env, simple command", &Config{}, nil, "echo hi", "echo hi"},
		{"with env prefix", &Config{}, []string{"A=1"}, "echo hi", "env A=1 echo hi"},
		{"trigger with inner quotes", &Config{}, nil,
			"lenos -- 'Run `ttal context` for your briefing'",
			"lenos -- 'Run `ttal context` for your briefing'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.BuildEnvShellCommand(tt.envParts, tt.cmd)
			if got != tt.want {
				t.Errorf("BuildEnvShellCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildEnvShellCommand_NestedSingleQuotes_RoundTrip(t *testing.T) {
	cfg := &Config{}
	trigger := "Run `ttal context` for your briefing, then act on the role prompt."
	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Fatalf("echo not found in PATH: %v", err)
	}
	inner := echoPath + " '" + trigger + "'"
	wrapped := cfg.BuildEnvShellCommand([]string{"FOO=bar"}, inner)
	out, err := exec.Command("/bin/sh", "-c", wrapped).Output()
	if err != nil {
		t.Fatalf("exec failed: %v\nstdout: %s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != trigger {
		t.Errorf("trigger corrupted in nested-single-quote round-trip:\n  want: %q\n  got:  %q", trigger, got)
	}
}
