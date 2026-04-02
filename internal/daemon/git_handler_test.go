package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitutil"
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
	if resp.OK {
		t.Error("expected OK=false on handler error")
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

// TestGitCredEnvForHost verifies that the shared credential helper resolves
// the correct token per host. Token resolution was previously tested via the
// removed tokenForHost — now delegated to gitutil.GitCredEnv.
func TestGitCredEnvForHost(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-token")
	t.Setenv("FORGEJO_TOKEN", "fg-token")

	tests := []struct {
		remoteURL  string
		wantToken  string
		wantHasEnv bool
	}{
		{"https://github.com/org/repo.git", "gh-token", true},
		{"git@github.com:org/repo.git", "gh-token", true},
		{"https://git.guion.io/org/repo.git", "fg-token", true},
		{"https://git.example.com/org/repo.git", "fg-token", true},
	}

	for _, tt := range tests {
		env := gitutil.GitCredEnv(tt.remoteURL, "")
		if tt.wantHasEnv && !gitutil.GitCredEnvHasToken(tt.remoteURL, "") {
			t.Errorf("GitCredEnvHasToken(%q) = false, want true", tt.remoteURL)
		}
		// GIT_TOKEN_INJECT=<token> is the last entry when credentials are injected.
		found := false
		for _, e := range env {
			if e == "GIT_TOKEN_INJECT="+tt.wantToken {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GitCredEnv(%q): GIT_TOKEN_INJECT=%q not found in env %v", tt.remoteURL, tt.wantToken, env)
		}
	}
}

func TestGitCredEnvForHost_EmptyToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("FORGEJO_TOKEN", "")

	// Without tokens: HasToken should be false, env should contain only GIT_TERMINAL_PROMPT=0.
	for _, url := range []string{
		"https://github.com/org/repo.git",
		"https://git.guion.io/org/repo.git",
	} {
		if gitutil.GitCredEnvHasToken(url, "") {
			t.Errorf("GitCredEnvHasToken(%q) = true with no tokens set", url)
		}
		env := gitutil.GitCredEnv(url, "")
		if len(env) != 1 || env[0] != "GIT_TERMINAL_PROMPT=0" {
			t.Errorf("GitCredEnv(%q) = %v, want [GIT_TERMINAL_PROMPT=0]", url, env)
		}
	}
}

func TestHTTPGitTag_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodPost, "/git/tag", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp GitTagResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Error("expected OK=false for bad JSON")
	}
}

func TestHTTPGitTag_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.gitTag = func(req GitTagRequest) GitTagResponse {
		return GitTagResponse{OK: false, Error: "tag failed"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(GitTagRequest{WorkDir: "/some/path", Tag: "v1.0.0"})
	req := httptest.NewRequest(http.MethodPost, "/git/tag", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler error → 500 with OK=false; detailed decode contract covered by TestHTTPGitPush_HandlerError.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTPGitTag_HappyPath(t *testing.T) {
	h := testHandlers(nil)
	var received GitTagRequest
	h.gitTag = func(req GitTagRequest) GitTagResponse {
		received = req
		return GitTagResponse{OK: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(GitTagRequest{WorkDir: "/some/project", Tag: "v1.0.0"})
	req := httptest.NewRequest(http.MethodPost, "/git/tag", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.WorkDir != "/some/project" {
		t.Errorf("expected WorkDir=/some/project, got %q", received.WorkDir)
	}
	if received.Tag != "v1.0.0" {
		t.Errorf("expected Tag=v1.0.0, got %q", received.Tag)
	}
}

func TestHandleGitTag_EmptyTag(t *testing.T) {
	resp := handleGitTag(GitTagRequest{WorkDir: "/some/path", Tag: ""})
	if resp.OK {
		t.Error("expected OK=false for empty tag")
	}
	if resp.Error != "tag must not be empty" {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestHandleGitTag_EmptyWorkDir(t *testing.T) {
	resp := handleGitTag(GitTagRequest{WorkDir: "", Tag: "v1.0.0"})
	if resp.OK {
		t.Error("expected OK=false for empty work_dir")
	}
	if resp.Error != "work_dir must not be empty" {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestHandleGitTag_UnregisteredProject(t *testing.T) {
	resp := handleGitTag(GitTagRequest{WorkDir: "/tmp/not-a-project", Tag: "v1.0.0"})
	if resp.OK {
		t.Error("expected OK=false for unregistered project")
	}
	if resp.Error != "tag only allowed for registered ttal projects" {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

// TestHandleGitTag_PathTraversal mirrors TestHandleGitPush_WorkDirValidation —
// ensures exact-match prevents adjacent-dir and parent-dir bypasses.
func TestHandleGitTag_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		workDir string
	}{
		{"adjacent directory bypass", "/tmp/my-project-evil"},
		{"parent directory", "/tmp"},
		{"path traversal", "/tmp/my-project/../other-project"},
		{"trailing slash", "/tmp/not-registered/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handleGitTag(GitTagRequest{WorkDir: tt.workDir, Tag: "v1.0.0"})
			if resp.OK {
				t.Error("expected OK=false for unregistered path")
			}
			if resp.Error != "tag only allowed for registered ttal projects" {
				t.Errorf("unexpected error: %q", resp.Error)
			}
		})
	}
}
