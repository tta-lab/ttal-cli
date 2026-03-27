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
	assert.Empty(t, cfg.PermissionsDeny)
}

func TestLoadSandbox_MalformedTOML(t *testing.T) {
	writeSandboxTOML(t, "this is not valid toml }{")
	cfg := LoadSandbox()
	// Must return clean zero value, not a partially-decoded config.
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.AllowWrite)
	assert.Empty(t, cfg.DenyRead)
	assert.Empty(t, cfg.AllowRead)
	assert.Empty(t, cfg.PermissionsDeny)
}

func TestLoadSandbox_ValidTOML(t *testing.T) {
	writeSandboxTOML(t, `
enabled = true
autoAllowBashIfSandboxed = false
allowWrite = ["/tmp"]
denyWrite = ["/tmp/protected"]
denyRead = ["~/"]
allowRead = ["~/.ssh", "~/.config/ttal"]
permissionsDeny = ["Read(~/.config/ttal/.env)", "Read(~/.ssh/id_ed25519)"]

[network]
allowedDomains = ["github.com", "*.guion.io"]
`)
	cfg := LoadSandbox()
	require.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.NotNil(t, cfg.AutoAllowBashIfSandboxed)
	assert.False(t, *cfg.AutoAllowBashIfSandboxed)
	assert.Equal(t, []string{"/tmp"}, cfg.AllowWrite)
	assert.Equal(t, []string{"/tmp/protected"}, cfg.DenyWrite)
	assert.Equal(t, []string{"~/"}, cfg.DenyRead)
	assert.Equal(t, []string{"~/.ssh", "~/.config/ttal"}, cfg.AllowRead)
	assert.Equal(t, []string{"Read(~/.config/ttal/.env)", "Read(~/.ssh/id_ed25519)"}, cfg.PermissionsDeny)
	assert.Equal(t, []string{"github.com", "*.guion.io"}, cfg.Network.AllowedDomains)
}

func TestLoadSandbox_AutoAllowBashNilWhenAbsent(t *testing.T) {
	writeSandboxTOML(t, `
enabled = true
allowWrite = ["/tmp"]
`)
	cfg := LoadSandbox()
	require.NotNil(t, cfg)
	assert.Nil(t, cfg.AutoAllowBashIfSandboxed, "AutoAllowBashIfSandboxed should be nil when not set")
}

func TestLoadSandbox_ExpandedPaths(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	writeSandboxTOML(t, `
enabled = true
allowWrite = ["~/mydata"]
denyWrite = ["~/protected"]
denyRead = ["~/"]
allowRead = ["~/.ssh"]
permissionsDeny = ["Read(~/.config/ttal/.env)"]
`)
	cfg := LoadSandbox()
	require.NotNil(t, cfg)

	assert.Contains(t, cfg.ExpandedAllowWrite(), filepath.Join(tmpHome, "mydata"))
	assert.Contains(t, cfg.ExpandedDenyWrite(), filepath.Join(tmpHome, "protected"))
	assert.Contains(t, cfg.ExpandedDenyRead(), tmpHome+"/")
	assert.Contains(t, cfg.ExpandedAllowRead(), filepath.Join(tmpHome, ".ssh"))

	expanded := cfg.ExpandedPermissionsDeny()
	assert.Contains(t, expanded, "Read("+filepath.Join(tmpHome, ".config/ttal/.env")+")")
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
