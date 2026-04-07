package temenos

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// startUnixServer starts a minimal HTTP server on a unix socket for testing.
// Returns the socket path. Uses os.MkdirTemp with a short prefix to stay within
// the macOS unix socket path limit (104 chars).
func startUnixServer(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	// Use a short prefix to avoid exceeding macOS's 104-char unix socket path limit.
	dir, err := os.MkdirTemp("", "tt")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	socketPath := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = os.RemoveAll(dir)
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

	token, err := c.RegisterSession(context.Background(), "coder", []string{"/tmp/work"}, []string{"/home/user"}, nil)
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

func TestRegisterSession_EnvEncoded(t *testing.T) {
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
		if req.Env == nil {
			http.Error(w, "env missing from request", http.StatusBadRequest)
			return
		}
		if v, ok := req.Env["TTAL_AGENT_NAME"]; !ok || v != "yuki" {
			http.Error(w, "TTAL_AGENT_NAME not set correctly", http.StatusBadRequest)
			return
		}
		if _, ok := req.Env["GITHUB_TOKEN"]; ok {
			http.Error(w, "GITHUB_TOKEN should not be in env", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registerResponse{Token: "envtoken123"})
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	env := map[string]string{
		"TTAL_AGENT_NAME": "yuki",
		"TTAL_JOB_ID":     "abc123",
	}
	token, err := c.RegisterSession(context.Background(), "yuki", []string{"/tmp/work"}, nil, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "envtoken123" {
		t.Errorf("expected token envtoken123, got %q", token)
	}
}

func TestRegisterSession_NonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/session/register", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	_, err := c.RegisterSession(context.Background(), "coder", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
	if !containsStatus(err.Error(), "500") {
		t.Errorf("expected status 500 in error, got: %v", err)
	}
}

func TestRegisterSession_EmptyToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/session/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registerResponse{Token: ""})
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	_, err := c.RegisterSession(context.Background(), "coder", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestDeleteSession_NonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	err := c.DeleteSession(context.Background(), "tok123")
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
	if !containsStatus(err.Error(), "404") {
		t.Errorf("expected status 404 in error, got: %v", err)
	}
}

func TestHealth_NonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	})

	socketPath := startUnixServer(t, mux)
	c := New(socketPath)

	err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
	if !containsStatus(err.Error(), "503") {
		t.Errorf("expected status 503 in error, got: %v", err)
	}
}

// containsStatus reports whether errMsg contains the given status code string.
func containsStatus(errMsg, code string) bool {
	return strings.Contains(errMsg, code)
}
