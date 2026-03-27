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

func TestPathsForPlane_AliasingIsSafe(t *testing.T) {
	// Calling PathsForPlane twice on the same SandboxConfig must not bleed
	// plane paths into the shared section (the classic Go append aliasing trap).
	cfg := &SandboxConfig{
		Shared:  SandboxPlane{ExtraPaths: []string{"/tmp:rw"}},
		Worker:  SandboxPlane{ExtraPaths: []string{"/tmp:rw"}},
		Manager: SandboxPlane{ExtraPaths: []string{"/tmp:rw"}},
	}

	worker := cfg.PathsForPlane("worker")
	manager := cfg.PathsForPlane("manager")

	// Both calls should return the same length (shared + one plane path).
	assert.Equal(t, len(worker), len(manager), "second call should not see paths from first call")
}

func TestPathsForPlane_UnknownPlane(t *testing.T) {
	cfg := &SandboxConfig{
		Shared: SandboxPlane{ExtraPaths: []string{"/tmp:rw"}},
	}
	paths := cfg.PathsForPlane("nonexistent")
	// Unknown plane falls back to shared only (no panic, no extra paths).
	assert.Len(t, paths, 1)
	assert.Equal(t, "/tmp:rw", paths[0])
}

func TestPathsForPlane_TildeExpansion(t *testing.T) {
	// Point HOME at a temp dir so ~ expansion resolves to a path we control,
	// and the subdir we create is guaranteed to pass PathsForPlane's os.Stat filter
	// on any machine (including CI runners where ~/.ttal doesn't exist).
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	subDir := filepath.Join(tmpHome, "mydata")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	cfg := &SandboxConfig{
		Shared: SandboxPlane{ExtraPaths: []string{"~/mydata:rw"}},
	}
	paths := cfg.PathsForPlane("shared")
	assert.Contains(t, paths, subDir+":rw")
}

func TestPathsForPlane_NonExistentFiltered(t *testing.T) {
	cfg := &SandboxConfig{
		Shared: SandboxPlane{ExtraPaths: []string{"/does/not/exist:ro"}},
	}
	paths := cfg.PathsForPlane("shared")
	assert.Empty(t, paths, "non-existent paths must be filtered out")
}

func TestLoadSandbox_FileNotExist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := LoadSandbox()
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Shared.ExtraPaths)
}

func TestLoadSandbox_MalformedTOML(t *testing.T) {
	writeSandboxTOML(t, "this is not valid toml }{")
	cfg := LoadSandbox()
	// Must return clean zero value, not a partially-decoded config.
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Shared.ExtraPaths)
	assert.Empty(t, cfg.Worker.ExtraPaths)
	assert.Empty(t, cfg.Manager.ExtraPaths)
}

func TestLoadSandbox_ValidTOML(t *testing.T) {
	writeSandboxTOML(t, `
[shared]
extra_paths = ["/tmp:rw"]

[worker]
extra_paths = ["/tmp:ro"]

[manager]
extra_paths = []
`)
	cfg := LoadSandbox()
	require.NotNil(t, cfg)
	assert.Equal(t, []string{"/tmp:rw"}, cfg.Shared.ExtraPaths)
	assert.Equal(t, []string{"/tmp:ro"}, cfg.Worker.ExtraPaths)
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
