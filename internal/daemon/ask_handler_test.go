package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
)

// sockCounter provides unique socket names so parallel tests don't collide.
var sockCounter atomic.Int64

func TestHandleAsk_InvalidMode(t *testing.T) {
	cfg := &config.Config{}
	handler := handleAsk(cfg)

	body, err := json.Marshal(map[string]string{
		"question": "test",
		"mode":     "invalid",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp SendResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "invalid mode")
}

func TestHandleAsk_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	handler := handleAsk(cfg)

	req := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// askAgentViaSocket runs AskAgent against a temporary unix-socket server
// running the provided handler. Returns all received events and any error.
func askAgentViaSocket(t *testing.T, handler http.HandlerFunc) ([]ask.Event, error) {
	t.Helper()

	// Use a short socket path to avoid hitting the unix socket path length limit (~104 bytes on macOS).
	sockPath := fmt.Sprintf("/tmp/ttal-test-%d.sock", sockCounter.Add(1))
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("skipping: cannot create unix socket (restricted environment): %v", err)
	}
	t.Cleanup(func() {
		ln.Close()
		os.Remove(sockPath)
	})

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ask" {
			handler(w, r)
			return
		}
		http.NotFound(w, r)
	})}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Close() })

	// Patch SocketPath for AskAgent by temporarily overriding the env var.
	t.Setenv("TTAL_SOCKET_PATH", sockPath)

	var events []ask.Event
	err = AskAgent(context.Background(), ask.Request{
		Question: "test",
		Mode:     ask.ModeGeneral,
	}, func(e ask.Event) {
		events = append(events, e)
	})
	return events, err
}

func TestAskAgent_StreamEndsWithoutTerminalEvent(t *testing.T) {
	// Server sends one delta event but no done/error — simulates daemon crash mid-stream.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(ask.Event{Type: ask.EventDelta, Text: "partial"})
		// Connection closes here without done/error event.
	})

	events, err := askAgentViaSocket(t, handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal event")
	assert.Len(t, events, 1) // the delta event was received before the error
}

func TestAskAgent_TerminalEventDone(t *testing.T) {
	// Server sends a done terminal event — clean finish.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(ask.Event{Type: ask.EventDelta, Text: "hello"})
		_ = enc.Encode(ask.Event{Type: ask.EventDone, Response: "hello"})
	})

	events, err := askAgentViaSocket(t, handler)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, ask.EventDone, events[1].Type)
}

func TestAskAgent_NonOKWithJSONBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeHTTPJSON(w, http.StatusBadRequest, SendResponse{OK: false, Error: "invalid mode: bogus"})
	})

	_, err := askAgentViaSocket(t, handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode: bogus")
}
