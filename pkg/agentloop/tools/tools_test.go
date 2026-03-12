package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

func TestNewBashTool_Constructs(t *testing.T) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	tool := NewBashTool(sbx)
	assert.NotNil(t, tool)
	assert.Equal(t, "bash", tool.Info().Name)
}

func TestNewReadURLTool_AllBackends(t *testing.T) {
	// BrowserGatewayBackend
	bgb := NewBrowserGatewayBackend("http://localhost:8080", nil)
	require.NotNil(t, bgb)
	tool1 := NewReadURLTool(bgb, 0)
	assert.Equal(t, "read_url", tool1.Info().Name)

	// DefuddleCLIBackend
	dcb := NewDefuddleCLIBackend()
	require.NotNil(t, dcb)
	tool2 := NewReadURLTool(dcb, 0)
	assert.Equal(t, "read_url", tool2.Info().Name)
}

func TestNewSearchWebTool_Constructs(t *testing.T) {
	tool := NewSearchWebTool(nil)
	assert.NotNil(t, tool)
	assert.Equal(t, "search_web", tool.Info().Name)
}

func TestNewDefaultToolSet(t *testing.T) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	backend := NewDefuddleCLIBackend()
	tools := NewDefaultToolSet(sbx, backend, nil, 0)

	require.Len(t, tools, 3)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Info().Name
	}
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "read_url")
	assert.Contains(t, names, "search_web")
}

func TestNewDefaultToolSet_WithAllowedPaths(t *testing.T) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	backend := NewDefuddleCLIBackend()
	dir := t.TempDir()
	tools := NewDefaultToolSet(sbx, backend, []string{dir}, 0)

	require.Len(t, tools, 7) // bash + read_url + search_web + read + read_md + glob + grep

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Info().Name
	}
	assert.Contains(t, names, "read")
	assert.Contains(t, names, "read_md")
	assert.Contains(t, names, "glob")
	assert.Contains(t, names, "grep")
}

func TestTruncateContent(t *testing.T) {
	short := "hello"
	assert.Equal(t, "hello", truncateContent(short))

	// Build a string > maxContentChars runes
	var big []rune
	for len(big) <= maxContentChars {
		big = append(big, []rune("abcde")...)
	}
	result := truncateContent(string(big))
	assert.LessOrEqual(t, len([]rune(result)), maxContentChars+100) // truncated
	assert.Contains(t, result, "[content truncated at 30,000 characters]")
}

// mockBackend is a simple ReadURLBackend for testing.
type mockBackend struct {
	content string
	err     error
}

func (m *mockBackend) Fetch(_ context.Context, _ string) (string, error) {
	return m.content, m.err
}

func TestReadURLBackend_Interface(t *testing.T) {
	var _ ReadURLBackend = &mockBackend{}
	var _ = NewDefuddleCLIBackend()
	var _ = NewBrowserGatewayBackend("", nil)
}
