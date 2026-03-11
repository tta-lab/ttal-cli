package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobTool_BasicPattern(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "foo.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bar.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "baz.txt"), []byte(""), 0o644))

	tool := NewGlobTool([]string{dir})
	content, isErr := runTool(t, tool, GlobParams{Pattern: "*.go", Path: dir})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "foo.go")
	assert.Contains(t, content, "bar.go")
	assert.NotContains(t, content, "baz.txt")
}

func TestGlobTool_RecursivePattern(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg", "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "deep.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.go"), []byte(""), 0o644))

	tool := NewGlobTool([]string{dir})
	content, isErr := runTool(t, tool, GlobParams{Pattern: "**/*.go", Path: dir})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "deep.go")
	assert.Contains(t, content, "root.go")
}

func TestGlobTool_PathOutsideAllowed(t *testing.T) {
	allowed := t.TempDir()
	other := t.TempDir()

	tool := NewGlobTool([]string{allowed})
	content, isErr := runTool(t, tool, GlobParams{Pattern: "*.go", Path: other})
	assert.True(t, isErr)
	assert.Contains(t, strings.ToLower(content), "access denied")
}

func TestGlobTool_EmptyPathSearchesAllAllowed(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "a.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "b.go"), []byte(""), 0o644))

	tool := NewGlobTool([]string{dir1, dir2})
	content, isErr := runTool(t, tool, GlobParams{Pattern: "*.go"})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "a.go")
	assert.Contains(t, content, "b.go")
}

func TestGlobTool_PathIsFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "afile.go")
	require.NoError(t, os.WriteFile(f, []byte(""), 0o644))

	tool := NewGlobTool([]string{dir})
	content, isErr := runTool(t, tool, GlobParams{Pattern: "*.go", Path: f})
	assert.True(t, isErr)
	assert.Contains(t, content, "not a directory")
}

func TestGlobTool_ResultsCapped(t *testing.T) {
	dir := t.TempDir()
	for i := range 210 {
		name := filepath.Join(dir, "file_"+string(rune('a'+i%26))+string(rune('a'+i/26))+".go")
		require.NoError(t, os.WriteFile(name, []byte(""), 0o644))
	}

	tool := NewGlobTool([]string{dir})
	content, isErr := runTool(t, tool, GlobParams{Pattern: "*.go", Path: dir})
	assert.False(t, isErr, content)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	assert.LessOrEqual(t, len(lines), 200)
}
