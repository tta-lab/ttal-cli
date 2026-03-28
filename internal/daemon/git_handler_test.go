package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHTTPGitPush_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodPost, "/git/push", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp GitPushResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Error("expected OK=false for bad JSON")
	}
}

func TestHTTPGitPush_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.gitPush = func(req GitPushRequest) GitPushResponse {
		return GitPushResponse{OK: false, Error: "push failed"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(GitPushRequest{WorkDir: "/some/path", Branch: "main"})
	req := httptest.NewRequest(http.MethodPost, "/git/push", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var resp GitPushResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "push failed" {
		t.Errorf("expected Error=push failed, got %q", resp.Error)
	}
}

func TestHTTPGitPush_HappyPath(t *testing.T) {
	h := testHandlers(nil)
	var received GitPushRequest
	h.gitPush = func(req GitPushRequest) GitPushResponse {
		received = req
		return GitPushResponse{OK: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(GitPushRequest{WorkDir: "/some/worktree", Branch: "worker/abc123"})
	req := httptest.NewRequest(http.MethodPost, "/git/push", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.WorkDir != "/some/worktree" {
		t.Errorf("expected WorkDir=/some/worktree, got %q", received.WorkDir)
	}
	if received.Branch != "worker/abc123" {
		t.Errorf("expected Branch=worker/abc123, got %q", received.Branch)
	}
}

func TestHandleGitPush_WorkDirValidation(t *testing.T) {
	home, _ := os.UserHomeDir()
	_ = home // ensure home is resolved

	resp := handleGitPush(GitPushRequest{
		WorkDir: "/tmp/some-other-repo",
		Branch:  "main",
	})

	if resp.OK {
		t.Error("expected OK=false for path outside worktrees")
	}
	if resp.Error != "push only allowed from ttal worktrees" {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestHandleGitPush_EmptyBranch(t *testing.T) {
	home, _ := os.UserHomeDir()
	worktreePath := home + "/.ttal/worktrees/test-worker"

	resp := handleGitPush(GitPushRequest{
		WorkDir: worktreePath,
		Branch:  "",
	})

	if resp.OK {
		t.Error("expected OK=false for empty branch")
	}
	if resp.Error != "branch must not be empty" {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestTokenForHost(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-token")
	t.Setenv("FORGEJO_TOKEN", "fg-token")

	tests := []struct {
		remoteURL string
		wantToken string
	}{
		{"https://github.com/org/repo.git", "gh-token"},
		{"git@github.com:org/repo.git", "gh-token"},
		{"https://git.guion.io/org/repo.git", "fg-token"},
		{"https://git.example.com/org/repo.git", "fg-token"},
	}

	for _, tt := range tests {
		got := tokenForHost(tt.remoteURL)
		if got != tt.wantToken {
			t.Errorf("tokenForHost(%q) = %q, want %q", tt.remoteURL, got, tt.wantToken)
		}
	}
}
