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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
)

func TestHandleSubagent_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	handler := handleSubagent(cfg)

	req := httptest.NewRequest(http.MethodPost, "/subagent/run", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp SendResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "invalid subagent JSON")
}

func TestHandleSubagent_MissingName(t *testing.T) {
	cfg := &config.Config{}
	handler := handleSubagent(cfg)

	body, err := json.Marshal(ask.SubagentRequest{Prompt: "test"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/subagent/run", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp SendResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp.Error, "name is required")
}

func TestHandleSubagent_MissingPrompt(t *testing.T) {
	cfg := &config.Config{}
	handler := handleSubagent(cfg)

	body, err := json.Marshal(ask.SubagentRequest{Name: "test-agent"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/subagent/run", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp SendResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp.Error, "prompt is required")
}

// runSubagentViaSocket runs daemon.RunSubagent against a temporary unix-socket server
// running the provided handler. Returns all received events and any error.
func runSubagentViaSocket(t *testing.T, handler http.HandlerFunc) ([]ask.Event, error) {
	t.Helper()

	sockPath := fmt.Sprintf("/tmp/ttal-test-subagent-%d.sock", sockCounter.Add(1))
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("skipping: cannot create unix socket (restricted environment): %v", err)
	}
	t.Cleanup(func() {
		ln.Close()
		os.Remove(sockPath)
	})

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subagent/run" {
			handler(w, r)
			return
		}
		http.NotFound(w, r)
	})}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Close() })

	t.Setenv("TTAL_SOCKET_PATH", sockPath)

	var events []ask.Event
	err = RunSubagent(context.Background(), ask.SubagentRequest{
		Name:   "test-agent",
		Prompt: "test prompt",
	}, func(e ask.Event) {
		events = append(events, e)
	})
	return events, err
}

func TestRunSubagent_StreamEndsWithoutTerminalEvent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(ask.Event{Type: ask.EventDelta, Text: "partial"})
	})

	events, err := runSubagentViaSocket(t, handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal event")
	assert.Len(t, events, 1)
}

func TestRunSubagent_TerminalEventDone(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(ask.Event{Type: ask.EventDelta, Text: "hello"})
		_ = enc.Encode(ask.Event{Type: ask.EventDone, Response: "hello"})
	})

	events, err := runSubagentViaSocket(t, handler)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, ask.EventDone, events[1].Type)
}

func TestRunSubagent_NonOKWithJSONBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeHTTPJSON(w, http.StatusBadRequest, SendResponse{OK: false, Error: "name is required"})
	})

	_, err := runSubagentViaSocket(t, handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}
