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

func TestDirectFetchBackend_PlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	backend := NewDirectFetchBackend(srv.Client())
	content, err := backend.Fetch(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "hello world", content)
}

func TestDirectFetchBackend_HTMLConverted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><h1>Title</h1><p>Body text</p></body></html>"))
	}))
	defer srv.Close()

	backend := NewDirectFetchBackend(srv.Client())
	content, err := backend.Fetch(context.Background(), srv.URL)
	require.NoError(t, err)
	// html-to-markdown should convert <h1> to # Title
	assert.Contains(t, content, "Title")
}

func TestDirectFetchBackend_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	backend := NewDirectFetchBackend(srv.Client())
	_, err := backend.Fetch(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestDirectFetchBackend_InvalidURL(t *testing.T) {
	backend := NewDirectFetchBackend(nil)
	_, err := backend.Fetch(context.Background(), "://invalid")
	require.Error(t, err)
}

func TestBrowserGatewayBackend_FallsBackOnGatewayError(t *testing.T) {
	// Gateway returns 500, should fall back to direct fetch of the target URL.
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("direct content"))
	}))
	defer targetSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gateway error", http.StatusInternalServerError)
	}))
	defer gatewaySrv.Close()

	backend := NewBrowserGatewayBackend(gatewaySrv.URL, gatewaySrv.Client())
	// Use gateway's client but target direct server — this tests fallback path.
	// We override the client to one that can reach both.
	backend2 := &browserGatewayBackend{gatewayURL: gatewaySrv.URL, client: targetSrv.Client()}
	content, err := backend2.Fetch(context.Background(), targetSrv.URL)
	require.NoError(t, err)
	assert.Equal(t, "direct content", content)
	_ = backend
}

func TestBrowserGatewayBackend_ContextCancelledNoFallback(t *testing.T) {
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a gateway error to trigger fallback path
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer gatewaySrv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	backend := &browserGatewayBackend{gatewayURL: gatewaySrv.URL, client: gatewaySrv.Client()}
	_, err := backend.Fetch(ctx, gatewaySrv.URL+"/target")
	// Should return context error, not attempt fallback
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
