package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
)

func TestCheckHumans_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgDir := tmpDir + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := cfgDir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte(`
[user]
  name = "neil"

[teams.default]
  frontend = "telegram"
  team_path = "/tmp"
  chat_id = "123456"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("HOME", tmpDir)

	section := checkHumans(false)
	assert.Equal(t, "Humans", section.Name)
	// Should have a warning about missing humans.toml
	found := false
	for _, ch := range section.Checks {
		if ch.Name == "humans_toml" && ch.Level == LevelWarn {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warn about missing humans.toml")
}

func TestCheckHumans_WithValidHumans(t *testing.T) {
	tmpDir := t.TempDir()
	cfgDir := tmpDir + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := cfgDir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte(`
[user]
  name = "neil"

[teams.default]
  frontend = "telegram"
  team_path = "/tmp"
  chat_id = "123456"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humansPath := filepath.Join(cfgDir, "humans.toml")
	neil := humanfs.Human{
		Alias:          "neil",
		Name:           "Neil",
		Age:            25,
		Pronouns:       "he/him",
		TelegramChatID: "123456",
		MatrixUserID:    "@neil:ttal.dev",
		Admin:          true,
	}
	if err := os.WriteFile(humansPath, []byte(`[neil]
name = "Neil"
age = 25
pronouns = "he/him"
telegram_chat_id = "123456"
matrix_user_id = "@neil:ttal.dev"
admin = true
`), 0o644); err != nil {
		t.Fatalf("write humans: %v", err)
	}
	_ = neil

	t.Setenv("HOME", tmpDir)

	section := checkHumans(false)
	assert.Equal(t, "Humans", section.Name)
	// Should have OK checks
	foundHumansOK := false
	foundAdminOK := false
	for _, ch := range section.Checks {
		if ch.Name == "humans_toml" && ch.Level == LevelOK {
			foundHumansOK = true
		}
		if ch.Name == "admin" && ch.Level == LevelOK {
			foundAdminOK = true
		}
	}
	assert.True(t, foundHumansOK, "expected OK for humans.toml")
	assert.True(t, foundAdminOK, "expected OK for admin")
}

func TestCheckHumans_NoAdmin(t *testing.T) {
	tmpDir := t.TempDir()
	cfgDir := tmpDir + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := cfgDir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte(`
[teams.default]
  frontend = "telegram"
  team_path = "/tmp"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humansPath := filepath.Join(cfgDir, "humans.toml")
	if err := os.WriteFile(humansPath, []byte(`[neil]
name = "Neil"
age = 25
telegram_chat_id = "123456"
admin = false
`), 0o644); err != nil {
		t.Fatalf("write humans: %v", err)
	}
	t.Setenv("HOME", tmpDir)

	section := checkHumans(false)
	found := false
	for _, ch := range section.Checks {
		if ch.Name == "admin" && ch.Level == LevelError && ch.Message == "no admin found (exactly one required)" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error about no admin")
}
