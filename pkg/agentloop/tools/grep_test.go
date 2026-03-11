package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrepTool_BasicSearch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("func Hello() {}\nfunc World() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("func Foo() {}\n"), 0o644))

	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "func Hello", Path: dir})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "Hello")
	assert.NotContains(t, content, "Foo")
}

func TestGrepTool_PathOutsideAllowed(t *testing.T) {
	allowed := t.TempDir()
	other := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(other, "x.txt"), []byte("secret"), 0o644))

	tool := NewGrepTool([]string{allowed})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "secret", Path: other})
	assert.True(t, isErr)
	assert.Contains(t, strings.ToLower(content), "access denied")
}

func TestGrepTool_EmptyPathSearchesAllAllowed(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "x.txt"), []byte("needle in dir1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "y.txt"), []byte("needle in dir2"), 0o644))

	tool := NewGrepTool([]string{dir1, dir2})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "needle"})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "needle in dir1")
	assert.Contains(t, content, "needle in dir2")
}

func TestGrepTool_GlobFilter(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("func main() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("func main() docs\n"), 0o644))

	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "func main", Path: dir, Glob: "*.go"})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "main.go")
	assert.NotContains(t, content, "README.md")
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "[invalid", Path: dir})
	assert.True(t, isErr)
	assert.Contains(t, strings.ToLower(content), "invalid")
}

func TestGrepTool_OutputTruncated(t *testing.T) {
	dir := t.TempDir()
	// Create a file with many matching lines.
	var sb strings.Builder
	for range 1000 {
		sb.WriteString("MATCH: " + strings.Repeat("x", 50) + "\n")
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.txt"), []byte(sb.String()), 0o644))

	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "MATCH", Path: dir})
	assert.False(t, isErr, content)
	assert.LessOrEqual(t, len([]rune(content)), maxContentChars+100)
}

func TestGrepTool_SingleFilePath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "only.go")
	require.NoError(t, os.WriteFile(f, []byte("func match() {}\nfunc other() {}\n"), 0o644))

	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "func match", Path: f})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "match")
	assert.NotContains(t, content, "other")
}

func TestGrepTool_SkippedFilesWarnInOutput(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "readable.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello\n"), 0o644))
	unreadable := filepath.Join(dir, "locked.txt")
	require.NoError(t, os.WriteFile(unreadable, []byte("secret\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: ".", Path: dir})
	assert.False(t, isErr, content)
	// Should warn about skipped files.
	assert.Contains(t, content, "Warning: skipped")
}

func TestGrepTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello world\n"), 0o644))

	tool := NewGrepTool([]string{dir})
	content, isErr := runTool(t, tool, GrepParams{Pattern: "zzznomatch", Path: dir})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "No matches")
}
