package doctor

import (
	"os"
	"path/filepath"
	"strings"
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
		MatrixUserID:   "@neil:ttal.dev",
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

func TestFixHumans_BootstrapsFromLegacy(t *testing.T) {
	// Unset USER so we actually test the [user].name read — not the env fallback
	t.Setenv("USER", "")

	tmpDir := t.TempDir()
	cfgDir := tmpDir + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := cfgDir + "/config.toml"
	config := `[user]
  name = "Fiona"

[teams.default]
  frontend = "telegram"
  team_path = "/tmp"
  chat_id = "845849177"

[teams.default.matrix]
  human_user_id = "@neil:ttal.dev"
`
	if err := os.WriteFile(cfgPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Set HOME so config.Path() resolves to the temp fixture
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	t.Cleanup(func() { t.Setenv("HOME", oldHome) })

	humansPath := cfgDir + "/humans.toml"

	// fixHumans should succeed even when humans.toml is absent
	err := fixHumans(humansPath)
	if err != nil {
		t.Fatalf("fixHumans failed: %v", err)
	}

	// Should write .generated file
	generatedPath := humansPath + ".generated"
	data, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}

	// Verify content
	content := string(data)
	if !strings.Contains(content, "[fiona]") {
		t.Errorf("generated content missing [fiona]:\n%s", content)
	}
	if !strings.Contains(content, `name = "Fiona"`) {
		t.Errorf("generated content missing name field:\n%s", content)
	}
	if !strings.Contains(content, `telegram_chat_id = "845849177"`) {
		t.Errorf("generated content missing telegram_chat_id:\n%s", content)
	}
	if !strings.Contains(content, `matrix_user_id = "@neil:ttal.dev"`) {
		t.Errorf("generated content missing matrix_user_id:\n%s", content)
	}
	if !strings.Contains(content, `admin = true`) {
		t.Errorf("generated content missing admin = true:\n%s", content)
	}
	if !strings.Contains(content, `age = 0`) {
		t.Errorf("generated content missing age = 0:\n%s", content)
	}
	if !strings.Contains(content, `pronouns = ""`) {
		t.Errorf("generated content missing pronouns:\n%s", content)
	}
}

func TestFixHumans_MissingChatID(t *testing.T) {
	t.Setenv("USER", "")
	tmpDir := t.TempDir()
	cfgDir := tmpDir + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := cfgDir + "/config.toml"
	config := `[user]
  name = "Neil"

[teams.default]
  frontend = "telegram"
  team_path = "/tmp"
`
	if err := os.WriteFile(cfgPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	t.Cleanup(func() { t.Setenv("HOME", oldHome) })

	humansPath := cfgDir + "/humans.toml"

	err := fixHumans(humansPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chat_id is empty")
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
