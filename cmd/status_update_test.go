package cmd

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
)

const validEnvelope = `{
  "version": 1,
  "event": "post_step",
  "step_index": 12,
  "session_id": "01HZSESSION",
  "model_id": "claude-sonnet-4-5",
  "context_window": 200000,
  "input_tokens": 14523,
  "output_tokens": 421,
  "total_tokens": 14944,
  "reasoning_tokens": 87,
  "cache_creation_tokens": 0,
  "cache_read_tokens": 8200,
  "context_used_pct": 7.47,
  "context_remaining_pct": 92.53,
  "timestamp": "2026-05-05T08:08:00Z"
}`

// startFakeDaemon mounts a /status/update handler on a TCP loopback socket.
// TCP is used because the CC sandbox blocks unix socket creation
// ("bind: operation not permitted").
func startFakeDaemon(t *testing.T) (*sync.Mutex, *daemon.StatusUpdateRequest) {
	t.Helper()
	mu := &sync.Mutex{}
	captured := &daemon.StatusUpdateRequest{}

	mux := http.NewServeMux()
	mux.HandleFunc("/status/update", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if err := json.NewDecoder(r.Body).Decode(captured); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(daemon.SendResponse{OK: true})
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}

	addr := ln.Addr().String()
	t.Setenv("TTAL_TEST_DAEMON_URL", "http://"+addr)

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	return mu, captured
}

func TestStatusUpdate_MissingAgent(t *testing.T) {
	err := statusUpdateFromReader(strings.NewReader(validEnvelope), "")
	if err == nil {
		t.Fatal("expected error when agent is empty")
	}
	want := "ttal status update: missing TTAL_AGENT_NAME env (must run inside a spawned manager-plane session)"
	if err.Error() != want {
		t.Errorf("error string mismatch:\n  got:  %q\n  want: %q", err.Error(), want)
	}
}

func TestStatusUpdate_EmptyStdin(t *testing.T) {
	err := statusUpdateFromReader(strings.NewReader(""), "mira")
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
	if !strings.Contains(err.Error(), "empty stdin") {
		t.Errorf("expected empty-stdin error, got: %v", err)
	}
}

func TestStatusUpdate_BadJSON(t *testing.T) {
	err := statusUpdateFromReader(strings.NewReader("not json"), "mira")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse envelope") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestStatusUpdate_VersionMismatch(t *testing.T) {
	err := statusUpdateFromReader(strings.NewReader(`{"version": 2, "session_id": "x"}`), "mira")
	if err == nil {
		t.Fatal("expected error for version 2")
	}
	if !strings.Contains(err.Error(), "unsupported envelope version 2") {
		t.Errorf("expected version-mismatch error, got: %v", err)
	}
}

func TestStatusUpdate_DaemonNotRunning(t *testing.T) {
	t.Setenv("TTAL_SOCKET_PATH", "/nonexistent/ttal-test.sock")

	err := statusUpdateFromReader(strings.NewReader(validEnvelope), "mira")
	if err == nil {
		t.Fatal("expected error when daemon socket is unreachable")
	}
	if !strings.Contains(err.Error(), "daemon not running") {
		t.Errorf("expected daemon-not-running error, got: %v", err)
	}
}

func TestStatusUpdate_HappyPath(t *testing.T) {
	mu, captured := startFakeDaemon(t)

	if err := statusUpdateFromReader(strings.NewReader(validEnvelope), "mira"); err != nil {
		t.Fatalf("happy path error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := daemon.StatusUpdateRequest{
		Type:                "statusUpdate",
		Agent:               "mira",
		ContextUsedPct:      7.47,
		ContextRemainingPct: 92.53,
		ModelID:             "claude-sonnet-4-5",
		SessionID:           "01HZSESSION",
	}
	if *captured != want {
		t.Errorf("captured request mismatch:\n  got:  %+v\n  want: %+v", *captured, want)
	}
}

func TestStatusUpdate_MinimalEnvelope(t *testing.T) {
	mu, captured := startFakeDaemon(t)

	if err := statusUpdateFromReader(strings.NewReader(`{"version": 1}`), "mira"); err != nil {
		t.Fatalf("minimal envelope error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := daemon.StatusUpdateRequest{
		Type:                "statusUpdate",
		Agent:               "mira",
		ContextUsedPct:      0,
		ContextRemainingPct: 0,
		ModelID:             "",
		SessionID:           "",
	}
	if *captured != want {
		t.Errorf("minimal envelope mapping mismatch:\n  got:  %+v\n  want: %+v", *captured, want)
	}
}

func TestStatusUpdate_ForwardCompatFields(t *testing.T) {
	t.Setenv("TTAL_SOCKET_PATH", "/nonexistent/ttal-fwd.sock")

	payload := `{"version": 1, "session_id": "s", "model_id": "m", "context_used_pct": 1.0, "context_remaining_pct": 99.0,
"weird_new_field": 42}`

	err := statusUpdateFromReader(strings.NewReader(payload), "mira")
	if err == nil {
		t.Fatal("expected daemon-not-running error")
	}
	if strings.Contains(err.Error(), "parse envelope") {
		t.Errorf("decoder rejected forward-compat field; expected lenient decode. err=%v", err)
	}
}

func TestStatusUpdate_SubcommandRegistered(t *testing.T) {
	for _, sub := range statusCmd.Commands() {
		if sub.Use == "update" {
			return
		}
	}
	t.Fatal("statusUpdateCmd not registered under statusCmd")
}
