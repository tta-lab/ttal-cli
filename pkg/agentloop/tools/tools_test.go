package tools

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

func TestNewBashTool_Constructs(t *testing.T) {
	sbx := &sandbox.Sandbox{AllowUnsandboxed: true}
	tool := NewBashTool(sbx)
	assert.NotNil(t, tool)
	assert.Equal(t, "bash", tool.Info().Name)
}

func TestNewWebFetchTool_AllBackends(t *testing.T) {
	// BrowserGatewayBackend
	bgb := NewBrowserGatewayBackend("http://localhost:8080", nil)
	require.NotNil(t, bgb)
	tool1 := NewWebFetchTool(bgb)
	assert.Equal(t, "web_fetch", tool1.Info().Name)

	// DefuddleCLIBackend
	dcb := NewDefuddleCLIBackend()
	require.NotNil(t, dcb)
	tool2 := NewWebFetchTool(dcb)
	assert.Equal(t, "web_fetch", tool2.Info().Name)

	// DirectFetchBackend
	dfb := NewDirectFetchBackend(nil)
	require.NotNil(t, dfb)
	tool3 := NewWebFetchTool(dfb)
	assert.Equal(t, "web_fetch", tool3.Info().Name)
}

func TestNewWebSearchTool_Constructs(t *testing.T) {
	tool := NewWebSearchTool(nil)
	assert.NotNil(t, tool)
	assert.Equal(t, "web_search", tool.Info().Name)
}

func TestNewDefaultToolSet(t *testing.T) {
	sbx := &sandbox.Sandbox{AllowUnsandboxed: true}
	backend := NewDirectFetchBackend(&http.Client{})
	tools := NewDefaultToolSet(sbx, backend)

	require.Len(t, tools, 3)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Info().Name
	}
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "web_fetch")
	assert.Contains(t, names, "web_search")
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

// mockBackend is a simple WebFetchBackend for testing.
type mockBackend struct {
	content string
	err     error
}

func (m *mockBackend) Fetch(_ context.Context, _ string) (string, error) {
	return m.content, m.err
}

func TestDirectFetchBackend_Interface(t *testing.T) {
	var _ WebFetchBackend = &mockBackend{}
	var _ = NewDirectFetchBackend(nil)
	var _ = NewDefuddleCLIBackend()
	var _ = NewBrowserGatewayBackend("", nil)
}
