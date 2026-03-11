package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const smallMD = "# Doc\n\n## Intro\n\nShort content.\n"

func largeMD() string {
	var sb strings.Builder
	sb.WriteString("# Big Doc\n\n")
	sb.WriteString("## Section A\n\n")
	for range 200 {
		sb.WriteString("This is a line of content that makes the file larger.\n")
	}
	sb.WriteString("\n## Section B\n\nMore content here.\n")
	return sb.String()
}

func TestReadMDTool_SmallFileReturnsFull(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "small.md")
	require.NoError(t, os.WriteFile(f, []byte(smallMD), 0o644))

	tool := NewReadMDTool([]string{dir}, 5000)
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "Short content.")
	assert.Contains(t, content, "## Intro")
}

func TestReadMDTool_LargeFileReturnsTree(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "large.md")
	require.NoError(t, os.WriteFile(f, []byte(largeMD()), 0o644))

	tool := NewReadMDTool([]string{dir}, 100) // low threshold to force tree
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f})
	assert.False(t, isErr, content)
	// Should return tree, not full content
	assert.Contains(t, content, "Section A")
	assert.Contains(t, content, "chars")
	assert.Contains(t, content, "section:")
}

func TestReadMDTool_ForceTreeOnSmallFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "small.md")
	require.NoError(t, os.WriteFile(f, []byte(smallMD), 0o644))

	tool := NewReadMDTool([]string{dir}, 5000)
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f, Tree: true})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "section:")
	assert.NotContains(t, content, "Short content.")
}

func TestReadMDTool_ForceFullOnLargeFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "large.md")
	md := largeMD()
	require.NoError(t, os.WriteFile(f, []byte(md), 0o644))

	tool := NewReadMDTool([]string{dir}, 10) // very low threshold
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f, Full: true})
	assert.False(t, isErr, content)
	// Should return full content (markdown, not tree)
	assert.Contains(t, content, "Section A")
	assert.NotContains(t, content, "section:") // no hint line
}

func TestReadMDTool_SectionExtraction(t *testing.T) {
	dir := t.TempDir()
	src := "# Document\n\n## Alpha\n\nalpha text\n\n## Beta\n\nbeta text\n"
	f := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(f, []byte(src), 0o644))

	// First get the tree to find section IDs.
	tool := NewReadMDTool([]string{dir}, 5000)
	treeContent, isErr := runTool(t, tool, ReadMDParams{FilePath: f, Tree: true})
	require.False(t, isErr, treeContent)

	// Extract section ID for "Alpha" from the tree output.
	// The tree line looks like: "└─ [XX] ## Alpha (N chars)"
	var alphaID string
	for _, line := range strings.Split(treeContent, "\n") {
		if strings.Contains(line, "Alpha") {
			start := strings.Index(line, "[")
			end := strings.Index(line, "]")
			if start >= 0 && end > start {
				alphaID = line[start+1 : end]
			}
		}
	}
	require.NotEmpty(t, alphaID, "could not find alpha section ID in tree: %s", treeContent)

	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f, Section: alphaID})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "alpha text")
	assert.NotContains(t, content, "beta text")
}

func TestReadMDTool_PathDenied(t *testing.T) {
	allowed := t.TempDir()
	other := t.TempDir()
	f := filepath.Join(other, "secret.md")
	require.NoError(t, os.WriteFile(f, []byte("# Secret\n"), 0o644))

	tool := NewReadMDTool([]string{allowed}, 5000)
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f})
	assert.True(t, isErr)
	assert.Contains(t, strings.ToLower(content), "access denied")
}

func TestReadMDTool_NoLineNumbers(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(f, []byte(smallMD), 0o644))

	tool := NewReadMDTool([]string{dir}, 5000)
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f})
	assert.False(t, isErr, content)
	// read_md should NOT add line numbers (unlike read)
	assert.NotContains(t, content, "1\t")
}

func TestReadMDTool_LargeFileNoHeadingsFallsBackToFull(t *testing.T) {
	dir := t.TempDir()
	// Plain text with no headings — large enough to exceed threshold.
	plain := strings.Repeat("just plain text\n", 50)
	f := filepath.Join(dir, "plain.md")
	require.NoError(t, os.WriteFile(f, []byte(plain), 0o644))

	tool := NewReadMDTool([]string{dir}, 10) // very low threshold
	content, isErr := runTool(t, tool, ReadMDParams{FilePath: f})
	assert.False(t, isErr, content)
	// Should return full content, not an empty tree.
	assert.Contains(t, content, "just plain text")
	assert.NotContains(t, content, "section:") // no hint line
}
