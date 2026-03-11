package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTool JSON-encodes params and invokes tool.Run, returning the content string and isError.
func runTool[P any](t *testing.T, tool fantasy.AgentTool, params P) (string, bool) {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{Input: string(input)})
	require.NoError(t, err)
	return resp.Content, resp.IsError
}

func TestReadTool_AllowedPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(f, []byte("line1\nline2\nline3\n"), 0o644))

	tool := NewReadTool([]string{dir})
	content, isErr := runTool(t, tool, ReadParams{FilePath: f})
	assert.False(t, isErr, "should not be an error: %s", content)
	assert.Contains(t, content, "line1")
	assert.Contains(t, content, "line2")
	assert.Contains(t, content, "line3")
}

func TestReadTool_DeniedPath(t *testing.T) {
	dir := t.TempDir()
	allowed := t.TempDir()
	f := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(f, []byte("secret"), 0o644))

	tool := NewReadTool([]string{allowed})
	content, isErr := runTool(t, tool, ReadParams{FilePath: f})
	assert.True(t, isErr, "should be denied")
	assert.Contains(t, strings.ToLower(content), "access denied")
}

func TestReadTool_DotDotTraversal(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	secret := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o644))

	// Only allow subdir — try to access parent via ".."
	tool := NewReadTool([]string{subdir})
	traversal := filepath.Join(subdir, "..", "secret.txt")
	content, isErr := runTool(t, tool, ReadParams{FilePath: traversal})
	assert.True(t, isErr, "traversal should be denied")
	assert.Contains(t, strings.ToLower(content), "access denied")
}

func TestReadTool_DirectoryRejected(t *testing.T) {
	dir := t.TempDir()
	tool := NewReadTool([]string{dir})
	content, isErr := runTool(t, tool, ReadParams{FilePath: dir})
	assert.True(t, isErr)
	assert.Contains(t, content, "is a directory")
}

func TestReadTool_LineNumbers(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "nums.txt")
	require.NoError(t, os.WriteFile(f, []byte("alpha\nbeta\ngamma\n"), 0o644))

	tool := NewReadTool([]string{dir})
	content, isErr := runTool(t, tool, ReadParams{FilePath: f})
	assert.False(t, isErr)
	// Should have line numbers (format: "     1\talpha")
	assert.Contains(t, content, "1\t")
	assert.Contains(t, content, "2\t")
	assert.Contains(t, content, "3\t")
}

func TestReadTool_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "multi.txt")
	lines := "a\nb\nc\nd\ne\n"
	require.NoError(t, os.WriteFile(f, []byte(lines), 0o644))

	tool := NewReadTool([]string{dir})
	// Offset=1 means skip first line (0-based), Limit=2 means read 2 lines
	content, isErr := runTool(t, tool, ReadParams{FilePath: f, Offset: 1, Limit: 2})
	assert.False(t, isErr)
	// Line numbers should start at offset+1 = 2
	assert.Contains(t, content, "2\tb")
	assert.Contains(t, content, "3\tc")
	assert.NotContains(t, content, "1\ta")
	assert.NotContains(t, content, "4\td")
}

func TestReadTool_SymlinkEscape(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("top secret"), 0o644))

	// Create a symlink inside the allowed dir pointing outside.
	link := filepath.Join(allowed, "escape")
	require.NoError(t, os.Symlink(secret, link))

	tool := NewReadTool([]string{allowed})
	content, isErr := runTool(t, tool, ReadParams{FilePath: link})
	assert.True(t, isErr, "symlink escaping allowed dir should be denied")
	assert.Contains(t, strings.ToLower(content), "access denied")
}

func TestReadTool_FileTooLarge(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "big.txt")
	big := make([]byte, 5*1024*1024+1)
	require.NoError(t, os.WriteFile(f, big, 0o644))

	tool := NewReadTool([]string{dir})
	content, isErr := runTool(t, tool, ReadParams{FilePath: f})
	assert.True(t, isErr)
	assert.Contains(t, strings.ToLower(content), "too large")
}

func TestReadTool_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	tool := NewReadTool([]string{dir})
	_, isErr := runTool(t, tool, ReadParams{FilePath: filepath.Join(dir, "missing.txt")})
	assert.True(t, isErr)
}
