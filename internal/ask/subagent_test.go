package ask

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tta-lab/ttal-cli/internal/config"
)

func TestCommandsForAccess_RW(t *testing.T) {
	cmds := CommandsForAccess("rw")
	// RW should include src edit command doc.
	found := false
	for _, c := range cmds {
		if c.Name == "src (edit)" {
			found = true
			break
		}
	}
	assert.True(t, found, "rw access should include src edit command")
}

func TestCommandsForAccess_RO(t *testing.T) {
	cmds := CommandsForAccess("ro")
	for _, c := range cmds {
		assert.NotEqual(t, "src (edit)", c.Name, "ro access should not include src edit command")
	}
	// Should still include basic commands.
	assert.NotEmpty(t, cmds)
}

func TestBuildSubagentSandboxPaths_DeduplicatesRWWins(t *testing.T) {
	sandbox := &config.SandboxConfig{
		AllowWrite: []string{"/rw/path"},
		AllowRead:  []string{"/rw/path", "/ro/path"},
	}

	paths := BuildSubagentSandboxPaths(sandbox, "/cwd", "ro")

	// /rw/path should be RW (not upgraded to RO by allowRead).
	for _, p := range paths {
		if p.Path == "/rw/path" {
			assert.False(t, p.ReadOnly, "/rw/path should be RW (write wins)")
		}
		if p.Path == "/ro/path" {
			assert.True(t, p.ReadOnly, "/ro/path should be RO")
		}
		if p.Path == "/cwd" {
			assert.True(t, p.ReadOnly, "CWD with ro access should be read-only")
		}
	}
}

func TestBuildSubagentSandboxPaths_CWDRWAccess(t *testing.T) {
	sandbox := &config.SandboxConfig{}

	paths := BuildSubagentSandboxPaths(sandbox, "/cwd", "rw")

	require := assert.New(t)
	require.Len(paths, 1)
	require.Equal("/cwd", paths[0].Path)
	require.False(paths[0].ReadOnly, "CWD with rw access should be read-write")
}

func TestBuildSubagentSandboxPaths_CWDUpgradesExistingEntry(t *testing.T) {
	sandbox := &config.SandboxConfig{
		AllowRead: []string{"/cwd"},
	}

	// CWD with rw access should upgrade the existing ro entry.
	paths := BuildSubagentSandboxPaths(sandbox, "/cwd", "rw")

	for _, p := range paths {
		if p.Path == "/cwd" {
			assert.False(t, p.ReadOnly, "CWD rw should upgrade existing ro entry")
		}
	}
}
