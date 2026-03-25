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
	require.Len(t, paths, 5)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".ttal")+":rw", paths[0])
	assert.Equal(t, filepath.Join(home, ".task")+":rw", paths[1])
	assert.Equal(t, filepath.Join(home, ".diary")+":rw", paths[2])
	assert.Equal(t, filepath.Join(home, ".local", "share", "flicknote")+":rw", paths[3])
	assert.Equal(t, filepath.Join(home, ".config", "ttal")+":ro", paths[4])
}

func TestWorkerTemenosEnv(t *testing.T) {
	env, err := WorkerTemenosEnv()
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
	// Verify only the 5 shared paths are present (no extra project paths)
	parts := strings.Split(strings.TrimPrefix(env[1], "TEMENOS_PATHS="), ",")
	assert.Len(t, parts, 5)
}

func TestReviewerTemenosEnv(t *testing.T) {
	env, err := ReviewerTemenosEnv()
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	assert.Equal(t, "ENABLE_TOOL_SEARCH=false", env[2])
	// Verify only the 5 shared paths are present
	parts := strings.Split(strings.TrimPrefix(env[1], "TEMENOS_PATHS="), ",")
	assert.Len(t, parts, 5)
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
	// Verify 5 shared + 2 project paths = 7 total
	parts := strings.Split(strings.TrimPrefix(env[1], "TEMENOS_PATHS="), ",")
	assert.Len(t, parts, 7)
}

func TestManagerTemenosEnv_NoProjects(t *testing.T) {
	env, err := ManagerTemenosEnv(nil)
	require.NoError(t, err)
	require.Len(t, env, 3)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	// Only the 5 shared paths
	parts := strings.Split(strings.TrimPrefix(env[1], "TEMENOS_PATHS="), ",")
	assert.Len(t, parts, 5)
}
