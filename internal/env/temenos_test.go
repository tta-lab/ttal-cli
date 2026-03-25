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
	env := WorkerTemenosEnv()
	require.Len(t, env, 2)
	assert.Equal(t, "TEMENOS_WRITE=true", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
	// Should NOT contain any :ro project paths
	assert.NotContains(t, env[1], "/Code/")
}

func TestReviewerTemenosEnv(t *testing.T) {
	env := ReviewerTemenosEnv()
	require.Len(t, env, 2)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.True(t, strings.HasPrefix(env[1], "TEMENOS_PATHS="))
}

func TestManagerTemenosEnv(t *testing.T) {
	projects := []string{"/proj/alpha", "/proj/beta"}
	env := ManagerTemenosEnv(projects)
	require.Len(t, env, 2)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	assert.Contains(t, env[1], "/proj/alpha:ro")
	assert.Contains(t, env[1], "/proj/beta:ro")
}

func TestManagerTemenosEnv_NoProjects(t *testing.T) {
	env := ManagerTemenosEnv(nil)
	require.Len(t, env, 2)
	assert.Equal(t, "TEMENOS_WRITE=false", env[0])
	// Only shared paths — the last shared path is :ro (.config/ttal), no project :ro paths after it
	// Verify no extra :ro paths beyond shared ones
	val := env[1]
	parts := strings.Split(strings.TrimPrefix(val, "TEMENOS_PATHS="), ",")
	assert.Len(t, parts, 5)
}
