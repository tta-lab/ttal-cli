package env

import (
	"fmt"
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
	// Use /tmp paths (always exist) so PathsForPlane's stat filter doesn't drop them.
	// This avoids a dependency on ~/.ttal or ~/.task existing on the test machine / CI runner.
	tmpA := filepath.Join(t.TempDir(), "shared-a")
	tmpB := filepath.Join(t.TempDir(), "shared-b")
	tmpW := filepath.Join(t.TempDir(), "worker-c")
	require.NoError(t, os.MkdirAll(tmpA, 0o755))
	require.NoError(t, os.MkdirAll(tmpB, 0o755))
	require.NoError(t, os.MkdirAll(tmpW, 0o755))

	writeSandboxTOML(t, fmt.Sprintf(`
[shared]
extra_paths = [%q, %q]

[worker]
extra_paths = [%q]

[manager]
extra_paths = []
`, tmpA+":rw", tmpB+":rw", tmpW+":rw"))

	env := WorkerTemenosEnv(nil)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Contains(t, env[1], tmpA+":rw")
	assert.Contains(t, env[1], tmpB+":rw")
	assert.Contains(t, env[1], tmpW+":rw")
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestWorkerTemenosEnvWithExtraPaths(t *testing.T) {
	writeSandboxTOML(t, "")
	env := WorkerTemenosEnv([]string{"/tmp/project-a", "/tmp/refs"})
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.Contains(t, env[1], "/tmp/project-a:ro")
	assert.Contains(t, env[1], "/tmp/refs:ro")
}

func TestReviewerTemenosEnv(t *testing.T) {
	writeSandboxTOML(t, `
[shared]
extra_paths = ["/tmp:rw"]

[worker]
extra_paths = ["/tmp:ro"]

[manager]
extra_paths = []
`)
	env := ReviewerTemenosEnv(nil)
	require.Len(t, env, 3)
	// Reviewers are read-only (TEMENOS_WRITE=false) but use the worker plane paths.
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	// Worker plane paths (not just shared) must appear — reviewer uses "worker" plane.
	assert.Contains(t, env[1], "/tmp:ro", "reviewer must include worker plane paths")
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
	env := ManagerTemenosEnv(projects)
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
	env := ManagerTemenosEnv(nil)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}
