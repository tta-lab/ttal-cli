package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedTemenosPaths(t *testing.T) {
	paths, err := sharedTemenosPaths()
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Contains(t, paths, filepath.Join(home, ".ttal")+":rw")
	assert.Contains(t, paths, filepath.Join(home, ".task")+":rw")
	assert.Contains(t, paths, filepath.Join(home, ".diary")+":rw")
	assert.Contains(t, paths, filepath.Join(home, ".local", "share", "flicknote")+":rw")
	assert.Contains(t, paths, filepath.Join(home, ".config", "ttal")+":ro")
	assert.Contains(t, paths, filepath.Join(home, ".config", "git")+":ro")
	assert.Contains(t, paths, filepath.Join(home, ".gitconfig")+":ro")
	assert.Contains(t, paths, filepath.Join(home, ".taskrc")+":ro")
}

func TestWorkerTemenosEnv(t *testing.T) {
	env, err := WorkerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestWorkerTemenosEnvWithExtraPaths(t *testing.T) {
	env, err := WorkerTemenosEnv([]string{"/tmp/project-a", "/tmp/refs"})
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.Contains(t, env[1], "/tmp/project-a:ro")
	assert.Contains(t, env[1], "/tmp/refs:ro")
}

func TestReviewerTemenosEnv(t *testing.T) {
	env, err := ReviewerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestManagerTemenosEnv(t *testing.T) {
	projects := []string{"/proj/alpha", "/proj/beta"}
	env, err := ManagerTemenosEnv(projects)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.Contains(t, env[1], "/proj/alpha:ro")
	assert.Contains(t, env[1], "/proj/beta:ro")
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}

func TestManagerTemenosEnv_NoProjects(t *testing.T) {
	env, err := ManagerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
}
