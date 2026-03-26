package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSandboxTOML writes a minimal sandbox.toml to a temp config dir and
// sets XDG_CONFIG_HOME so config.DefaultConfigDir() picks it up.
func writeSandboxTOML(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	ttalDir := filepath.Join(dir, "ttal")
	require.NoError(t, os.MkdirAll(ttalDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ttalDir, "sandbox.toml"), []byte(content), 0o644))
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestWorkerTemenosEnv(t *testing.T) {
	writeSandboxTOML(t, `
[shared]
extra_paths = ["~/.ttal:rw", "~/.task:rw"]

[worker]
extra_paths = ["~/Library/Caches/go-build:rw"]

[manager]
extra_paths = []
`)
	env, err := WorkerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))

	home, _ := os.UserHomeDir()
	assert.Contains(t, env[1], home+"/.ttal:rw")
	assert.Contains(t, env[1], home+"/.task:rw")
	assert.Contains(t, env[1], home+"/Library/Caches/go-build:rw")
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestWorkerTemenosEnvWithExtraPaths(t *testing.T) {
	writeSandboxTOML(t, "")
	env, err := WorkerTemenosEnv([]string{"/tmp/project-a", "/tmp/refs"})
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.Contains(t, env[1], "/tmp/project-a:ro")
	assert.Contains(t, env[1], "/tmp/refs:ro")
}

func TestReviewerTemenosEnv(t *testing.T) {
	writeSandboxTOML(t, "")
	env, err := ReviewerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestManagerTemenosEnv(t *testing.T) {
	writeSandboxTOML(t, `
[shared]
extra_paths = ["~/.ttal:rw"]

[worker]
extra_paths = ["~/Library/Caches/go-build:rw"]

[manager]
extra_paths = []
`)
	projects := []string{"/proj/alpha", "/proj/beta"}
	env, err := ManagerTemenosEnv(projects)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.Contains(t, env[1], "/proj/alpha:ro")
	assert.Contains(t, env[1], "/proj/beta:ro")
	// worker-only paths must NOT appear in manager env
	assert.NotContains(t, env[1], "go-build")
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestManagerTemenosEnv_NoProjects(t *testing.T) {
	writeSandboxTOML(t, "")
	env, err := ManagerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}
