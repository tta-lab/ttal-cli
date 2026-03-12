package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserGatewayBackend_GatewayError_ReturnsError(t *testing.T) {
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gateway error", http.StatusInternalServerError)
	}))
	defer gatewaySrv.Close()

	backend := &browserGatewayBackend{gatewayURL: gatewaySrv.URL, client: gatewaySrv.Client()}
	_, err := backend.Fetch(context.Background(), "https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestBrowserGatewayBackend_ContextCancelled_ReturnsError(t *testing.T) {
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer gatewaySrv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	backend := &browserGatewayBackend{gatewayURL: gatewaySrv.URL, client: gatewaySrv.Client()}
	_, err := backend.Fetch(ctx, gatewaySrv.URL+"/target")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBrowserGatewayBackend_SuccessfulExtraction(t *testing.T) {
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"My Title","author":"Jane","content":"Body text here"}`))
	}))
	defer gatewaySrv.Close()

	backend := &browserGatewayBackend{gatewayURL: gatewaySrv.URL, client: gatewaySrv.Client()}
	content, err := backend.Fetch(context.Background(), "https://example.com")
	require.NoError(t, err)
	assert.Contains(t, content, "My Title")
	assert.Contains(t, content, "Jane")
	assert.Contains(t, content, "Body text here")
}

func TestTruncateContent_WithinLimit(t *testing.T) {
	s := "short string"
	assert.Equal(t, s, truncateContent(s))
}

func TestTruncateContent_Truncated(t *testing.T) {
	// Build a string longer than maxContentChars runes
	long := strings.Repeat("a", maxContentChars+100)
	result := truncateContent(long)
	assert.LessOrEqual(t, len([]rune(result)), maxContentChars+50)
	assert.Contains(t, result, "[content truncated at 30,000 characters]")
}

// --- read_url tool tests (tree/section/full modes + cache) ---

func makeMockServer(content string, contentType string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(content))
	}))
}

func TestReadURLTool_SmallContentReturnsFull(t *testing.T) {
	srv := makeMockServer("# Title\n\nShort content.", "text/plain")
	defer srv.Close()

	backend := &mockReadURLBackend{content: "# Title\n\nShort content."}
	tool := NewReadURLTool(backend, 5000)
	content, isErr := runTool(t, tool, ReadURLParams{URL: srv.URL})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "Short content.")
}

func TestReadURLTool_LargeContentReturnsTree(t *testing.T) {
	md := "# Big Doc\n\n## Section A\n\n" + strings.Repeat("x ", 300) + "\n\n## Section B\n\nmore\n"
	backend := &mockReadURLBackend{content: md}
	tool := NewReadURLTool(backend, 100) // low threshold

	content, isErr := runTool(t, tool, ReadURLParams{URL: "https://example.com"})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "Section A")
	assert.Contains(t, content, "chars")
	assert.Contains(t, content, "section:")
}

func TestReadURLTool_ForceFull(t *testing.T) {
	md := "# Doc\n\n## Big Section\n\n" + strings.Repeat("content ", 200) + "\n"
	backend := &mockReadURLBackend{content: md}
	tool := NewReadURLTool(backend, 10) // very low threshold

	content, isErr := runTool(t, tool, ReadURLParams{URL: "https://example.com", Full: true})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "content content content")
	assert.NotContains(t, content, "section:") // no hint in full mode
}

func TestReadURLTool_ForceTree(t *testing.T) {
	md := "# Doc\n\n## Section\n\nshort\n"
	backend := &mockReadURLBackend{content: md}
	tool := NewReadURLTool(backend, 5000)

	content, isErr := runTool(t, tool, ReadURLParams{URL: "https://example.com", Tree: true})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "section:")
}

func TestReadURLTool_SectionExtraction(t *testing.T) {
	md := "# Doc\n\n## Alpha\n\nalpha text\n\n## Beta\n\nbeta text\n"
	backend := &mockReadURLBackend{content: md}
	tool := NewReadURLTool(backend, 5000)

	// First get the tree to find section IDs.
	treeContent, isErr := runTool(t, tool, ReadURLParams{URL: "https://example.com", Tree: true})
	require.False(t, isErr, treeContent)

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
	require.NotEmpty(t, alphaID, "could not find Alpha section ID in: %s", treeContent)

	content, isErr := runTool(t, tool, ReadURLParams{URL: "https://example.com", Section: alphaID})
	assert.False(t, isErr, content)
	assert.Contains(t, content, "alpha text")
	assert.NotContains(t, content, "beta text")
}

func TestReadURLTool_CacheHit(t *testing.T) {
	fetchCount := 0
	backend := &countingBackend{content: "# Doc\n\nshort", fetchCount: &fetchCount}
	tool := NewReadURLTool(backend, 5000)

	// Fetch twice — should only hit backend once.
	_, _ = runTool(t, tool, ReadURLParams{URL: "https://example.com"})
	_, _ = runTool(t, tool, ReadURLParams{URL: "https://example.com"})
	assert.Equal(t, 1, fetchCount)
}

func TestReadURLTool_CacheMissOnSection(t *testing.T) {
	md := "# Doc\n\n## Alpha\n\nalpha text\n\n## Beta\n\nbeta text\n"
	fetchCount := 0
	backend := &countingBackend{content: md, fetchCount: &fetchCount}
	tool := NewReadURLTool(backend, 5000)

	// Get tree first (fetch 1), then section (cache hit — no fetch 2).
	treeContent, _ := runTool(t, tool, ReadURLParams{URL: "https://example.com", Tree: true})

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
	require.NotEmpty(t, alphaID)

	_, _ = runTool(t, tool, ReadURLParams{URL: "https://example.com", Section: alphaID})
	assert.Equal(t, 1, fetchCount, "section call should use cache, not re-fetch")
}

func TestReadURLTool_NoHeadingsFallsBackToFull(t *testing.T) {
	md := "just plain text with no headings at all"
	backend := &mockReadURLBackend{content: md}
	tool := NewReadURLTool(backend, 10) // low threshold forces tree mode

	content, isErr := runTool(t, tool, ReadURLParams{URL: "https://example.com"})
	assert.False(t, isErr, content)
	// No headings — should return full content instead of empty tree.
	assert.Contains(t, content, "just plain text")
}

// --- mock backends ---

type mockReadURLBackend struct {
	content string
	err     error
}

func (m *mockReadURLBackend) Fetch(_ context.Context, _ string) (string, error) {
	return m.content, m.err
}

type countingBackend struct {
	content    string
	fetchCount *int
}

func (c *countingBackend) Fetch(_ context.Context, _ string) (string, error) {
	*c.fetchCount++
	return c.content, nil
}
