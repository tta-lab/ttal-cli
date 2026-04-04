package temenos

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// startUnixServer starts a minimal HTTP server on a unix socket for testing.
// Returns the socket path and a cleanup function.
func startUnixServer(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = os.Remove(socketPath)
	})
	return socketPath
}

func TestRegisterSession(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/session/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registerResponse{Token: "abc123token"})
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	token, err := c.RegisterSession(context.Background(), "coder", []string{"/tmp/work"}, []string{"/home/user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc123token" {
		t.Errorf("expected token abc123token, got %q", token)
	}
}

func TestDeleteSession(t *testing.T) {
	deleted := ""
	mux := http.NewServeMux()
	mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		deleted = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	if err := c.DeleteSession(context.Background(), "tok123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != "/session/tok123" {
		t.Errorf("expected DELETE /session/tok123, got %q", deleted)
	}
}

func TestHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_DefaultSocketPath(t *testing.T) {
	c := New("")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".temenos", "daemon.sock")
	if c.socketPath != want {
		t.Errorf("expected socket path %q, got %q", want, c.socketPath)
	}
}
