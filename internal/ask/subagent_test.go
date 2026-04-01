package ask

import (
	"path/filepath"
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

	paths := BuildSubagentSandboxPaths(sandbox, "/cwd", "ro", nil)

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

	paths := BuildSubagentSandboxPaths(sandbox, "/cwd", "rw", nil)

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
	paths := BuildSubagentSandboxPaths(sandbox, "/cwd", "rw", nil)

	for _, p := range paths {
		if p.Path == "/cwd" {
			assert.False(t, p.ReadOnly, "CWD rw should upgrade existing ro entry")
		}
	}
}
func TestBuildSubagentSandboxPaths_SkipsRelativePaths(t *testing.T) {
	sandbox := &config.SandboxConfig{
		AllowRead:  []string{".", "../foo", "./bar", "/tmp/read"},
		AllowWrite: []string{"/tmp/write"},
	}

	paths := BuildSubagentSandboxPaths(sandbox, "/home/test", "rw", nil)

	// All returned paths should be absolute.
	for _, p := range paths {
		assert.True(t, filepath.IsAbs(p.Path), "all paths should be absolute, got: %s", p.Path)
	}

	// CWD should be present with rw access.
	foundCWD := false
	// /tmp/read should be present.
	foundRead := false
	// /tmp/write should be present.
	foundWrite := false

	for _, p := range paths {
		if p.Path == "/home/test" {
			foundCWD = true
			assert.False(t, p.ReadOnly, "CWD with rw access should be read-write")
		}
		if p.Path == "/tmp/read" {
			foundRead = true
			assert.True(t, p.ReadOnly, "/tmp/read should be read-only")
		}
		if p.Path == "/tmp/write" {
			foundWrite = true
			assert.False(t, p.ReadOnly, "/tmp/write should be read-write")
		}
	}

	assert.True(t, foundCWD, "/home/test (CWD) should be present")
	assert.True(t, foundRead, "/tmp/read should be present")
	assert.True(t, foundWrite, "/tmp/write should be present")

	// Should not contain relative paths like '.', '../foo', './bar'.
	for _, p := range paths {
		assert.NotEqual(t, ".", p.Path)
		assert.NotEqual(t, "../foo", p.Path)
		assert.NotEqual(t, "./bar", p.Path)
	}
}

func TestBuildSubagentSandboxPaths_CWDIsFirstMount(t *testing.T) {
	sandbox := &config.SandboxConfig{
		AllowWrite: []string{"/other/write"},
		AllowRead:  []string{"/other/read"},
	}

	paths := BuildSubagentSandboxPaths(sandbox, "/project/dir", "rw", nil)

	require := assert.New(t)
	require.NotEmpty(paths)
	require.Equal("/project/dir", paths[0].Path, "CWD must be first so temenos uses it as WorkingDir")
}
