package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSandboxTOML(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	ttalDir := filepath.Join(dir, "ttal")
	require.NoError(t, os.MkdirAll(ttalDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ttalDir, "sandbox.toml"), []byte(content), 0o644))
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestLoadSandbox_FileNotExist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := LoadSandbox()
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.AllowWrite)
	assert.Empty(t, cfg.DenyRead)
	assert.Empty(t, cfg.AllowRead)
}

func TestLoadSandbox_MalformedTOML(t *testing.T) {
	writeSandboxTOML(t, "this is not valid toml }{")
	cfg := LoadSandbox()
	// Must return clean zero value, not a partially-decoded config.
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.AllowWrite)
	assert.Empty(t, cfg.DenyRead)
	assert.Empty(t, cfg.AllowRead)
}

func TestLoadSandbox_ValidTOML(t *testing.T) {
	writeSandboxTOML(t, `
enabled = true
allowWrite = ["/tmp"]
denyRead = ["~/.config/ttal/.env"]
allowRead = []

[network]
allowedDomains = ["github.com", "*.guion.io"]
`)
	cfg := LoadSandbox()
	require.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, []string{"/tmp"}, cfg.AllowWrite)
	assert.Equal(t, []string{"~/.config/ttal/.env"}, cfg.DenyRead)
	assert.Equal(t, []string{"github.com", "*.guion.io"}, cfg.Network.AllowedDomains)
}

func TestLoadSandbox_ExpandedPaths(t *testing.T) {
	// Point HOME at a temp dir so ~ expansion resolves to a path we control.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	writeSandboxTOML(t, `
enabled = true
allowWrite = ["~/mydata"]
denyRead = ["~/.config/ttal/.env"]
`)
	cfg := LoadSandbox()
	require.NotNil(t, cfg)

	expanded := cfg.ExpandedAllowWrite()
	assert.Contains(t, expanded, filepath.Join(tmpHome, "mydata"))

	expandedDeny := cfg.ExpandedDenyRead()
	assert.Contains(t, expandedDeny, filepath.Join(tmpHome, ".config/ttal/.env"))
}

func TestDefaultConfigDir_XGDBranch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	got := DefaultConfigDir()
	assert.Equal(t, filepath.Join(tmp, "ttal"), got)
}

func TestDefaultConfigDir_HomeBranch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	got := DefaultConfigDir()
	assert.Equal(t, filepath.Join(home, ".config", "ttal"), got)
}
