package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

func TestHTTPGitPull_HappyPath(t *testing.T) {
	h := testHandlers(nil)
	var received GitPullRequest
	h.gitPull = func(req GitPullRequest) GitPullResponse {
		received = req
		return GitPullResponse{OK: true, Action: GitPullActionPulledBranch}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(GitPullRequest{
		WorkDir:       "/some/worktree",
		Branch:        "feature/x",
		DefaultBranch: "main",
		Mode:          GitPullModeBranch,
	})
	req := httptest.NewRequest(http.MethodPost, "/git/pull", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.WorkDir != "/some/worktree" || received.Branch != "feature/x" || received.Mode != GitPullModeBranch {
		t.Errorf("unexpected request: %+v", received)
	}
}

func TestBuildGitPushArgs(t *testing.T) {
	cases := []struct {
		name string
		req  GitPushRequest
		want []string
	}{
		{
			"no force",
			GitPushRequest{WorkDir: "/wd", Branch: "feature/x"},
			[]string{"-C", "/wd", "push", "-u", "origin", "feature/x"},
		},
		{
			"force appends --force-with-lease",
			GitPushRequest{WorkDir: "/wd", Branch: "feature/x", Force: true},
			[]string{"-C", "/wd", "push", "-u", "origin", "feature/x", "--force-with-lease"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildGitPushArgs(tc.req)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buildGitPushArgs() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildGitPullCommands(t *testing.T) {
	cases := []struct {
		name string
		req  GitPullRequest
		want [][]string
	}{
		{
			"default branch",
			GitPullRequest{WorkDir: "/wd", Branch: "main", DefaultBranch: "main", Mode: GitPullModeDefault},
			[][]string{{"-C", "/wd", "pull", "--ff-only", "origin", "main"}},
		},
		{
			"current branch",
			GitPullRequest{WorkDir: "/wd", Branch: "feature/x", DefaultBranch: "main", Mode: GitPullModeBranch},
			[][]string{{"-C", "/wd", "pull", "--ff-only", "origin", "feature/x"}},
		},
		{
			"cleanup merged branch",
			GitPullRequest{WorkDir: "/wd", Branch: "feature/x", DefaultBranch: "main", Mode: GitPullModeCleanupMerged},
			[][]string{
				{"-C", "/wd", "switch", "main"},
				{"-C", "/wd", "pull", "--ff-only", "origin", "main"},
				{"-C", "/wd", "branch", "-D", "feature/x"},
				{"-C", "/wd", "push", "origin", "--delete", "feature/x"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildGitPullCommands(tc.req)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buildGitPullCommands() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseLocalAheadCount(t *testing.T) {
	got, err := parseLocalAheadCount("3\n")
	if err != nil {
		t.Fatalf("parseLocalAheadCount: %v", err)
	}
	if got != 3 {
		t.Fatalf("parseLocalAheadCount = %d, want 3", got)
	}
}

func TestParseLocalAheadCountRejectsBadOutput(t *testing.T) {
	_, err := parseLocalAheadCount("not-a-number\n")
	if err == nil {
		t.Fatal("expected error for invalid rev-list output")
	}
}

func TestEnsureBranchNotAheadOfOriginBlocksLocalCommits(t *testing.T) {
	repo := initAheadOfOriginRepo(t)

	err := ensureBranchNotAheadOfOrigin(repo, "feature/x")
	if err == nil {
		t.Fatal("expected ahead branch to be rejected")
	}
	if !strings.Contains(err.Error(), "has 1 local commit(s) not on origin/feature/x") {
		t.Fatalf("error = %q, want ahead-count message", err)
	}
}

func TestEnsureCleanBranchForCleanupAllowsDeletedRemoteBranch(t *testing.T) {
	repo := initRepoWithDeletedOriginBranch(t)

	err := ensureCleanBranchForCleanup(repo, "feature/x", nil)
	if err != nil {
		t.Fatalf("expected deleted remote branch to be cleanup-safe: %v", err)
	}
}

func TestRefreshRemoteBranchForCleanupUsesCredentialEnv(t *testing.T) {
	t.Setenv("GO_WANT_GIT_COMMAND_HELPER", "1")
	orig := gitCommandContext
	t.Cleanup(func() { gitCommandContext = orig })
	gitCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		helperArgs := append([]string{"-test.run=TestGitCommandHelper", "--", name}, args...)
		return exec.CommandContext(ctx, os.Args[0], helperArgs...)
	}

	_, err := refreshRemoteBranchForCleanup("/work", "feature/x", []string{"TTAL_TEST_CRED=yes"})
	if err != nil {
		t.Fatalf("expected credential env to be passed to cleanup fetch: %v", err)
	}
}

func TestGitCommandHelper(t *testing.T) {
	if os.Getenv("GO_WANT_GIT_COMMAND_HELPER") != "1" {
		return
	}
	args := os.Args
	sep := slices.Index(args, "--")
	if sep == -1 || sep+1 >= len(args) {
		os.Exit(2)
	}
	gitArgs := args[sep+1:]
	if slices.Contains(gitArgs, "fetch") {
		if os.Getenv("TTAL_TEST_CRED") != "yes" {
			os.Exit(3)
		}
		os.Exit(0)
	}
	if slices.Contains(gitArgs, "show-ref") {
		os.Exit(1)
	}
	os.Exit(2)
}

func TestEnsureCleanBranchForCleanupBlocksDirtyWorktree(t *testing.T) {
	repo := initSyncedBranchRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\ndirty\n"), 0o600); err != nil {
		t.Fatalf("dirty file: %v", err)
	}

	err := ensureCleanBranchForCleanup(repo, "feature/x", nil)
	if err == nil {
		t.Fatal("expected dirty worktree to be rejected")
	}
	if !strings.Contains(err.Error(), "worktree has uncommitted changes") {
		t.Fatalf("error = %q, want dirty-worktree message", err)
	}
}

func TestIsMissingRemoteBranchDelete(t *testing.T) {
	missing := []string{
		"error: unable to delete 'feature/x': remote ref does not exist",
		"error: unable to delete 'feature/x': remote ref does not exist\nerror: failed to push some refs",
	}
	for _, out := range missing {
		if !isMissingRemoteBranchDelete(out) {
			t.Errorf("isMissingRemoteBranchDelete(%q) = false, want true", out)
		}
	}

	if isMissingRemoteBranchDelete("fatal: Authentication failed") {
		t.Error("auth failures must not be treated as missing remote branches")
	}
}

func initAheadOfOriginRepo(t *testing.T) string {
	t.Helper()
	repo := initSyncedBranchRepo(t)

	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\nlocal\n"), 0o600); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	runGitTestCmd(t, repo, "add", "file.txt")
	runGitTestCmd(t, repo, "commit", "-m", "local")

	return repo
}

func initRepoWithDeletedOriginBranch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	remote := filepath.Join(dir, "origin.git")
	repo := filepath.Join(dir, "repo")

	runGitTestCmd(t, dir, "init", "--bare", remote)
	runGitTestCmd(t, dir, "clone", remote, repo)
	runGitTestCmd(t, repo, "config", "user.email", "test@example.com")
	runGitTestCmd(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitTestCmd(t, repo, "add", "file.txt")
	runGitTestCmd(t, repo, "commit", "-m", "base")
	runGitTestCmd(t, repo, "branch", "-M", "main")
	runGitTestCmd(t, repo, "push", "-u", "origin", "main")
	runGitTestCmd(t, repo, "switch", "-c", "feature/x")
	runGitTestCmd(t, repo, "push", "-u", "origin", "feature/x")
	runGitTestCmd(t, repo, "push", "origin", "--delete", "feature/x")

	return repo
}

func initSyncedBranchRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")

	runGitTestCmd(t, dir, "init", repo)
	runGitTestCmd(t, repo, "config", "user.email", "test@example.com")
	runGitTestCmd(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitTestCmd(t, repo, "add", "file.txt")
	runGitTestCmd(t, repo, "commit", "-m", "base")
	runGitTestCmd(t, repo, "switch", "-c", "feature/x")
	runGitTestCmd(t, repo, "update-ref", "refs/remotes/origin/feature/x", "HEAD")

	return repo
}

func runGitTestCmd(t *testing.T, workDir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestHandleGitPush_ForceOnProtectedBranchBlocked(t *testing.T) {
	for _, branch := range []string{"main", "master"} {
		t.Run(branch, func(t *testing.T) {
			resp := handleGitPush(GitPushRequest{
				WorkDir: "/tmp/whatever", // never reached
				Branch:  branch,
				Force:   true,
			})
			if resp.OK {
				t.Fatalf("expected OK=false for force push to %s", branch)
			}
			wantErr := fmt.Sprintf("push to %s blocked — use a feature branch and PR", branch)
			if resp.Error != wantErr {
				t.Errorf("error = %q, want %q", resp.Error, wantErr)
			}
		})
	}
}

func TestIsProtectedBranch(t *testing.T) {
	protected := []string{"main", "master"}
	allowed := []string{"develop", "feature/x", "release/v1", ""}

	for _, b := range protected {
		if !isProtectedBranch(b) {
			t.Errorf("isProtectedBranch(%q) = false, want true", b)
		}
	}
	for _, b := range allowed {
		if isProtectedBranch(b) {
			t.Errorf("isProtectedBranch(%q) = true, want false", b)
		}
	}
}

func TestHandleGitPush_NormalPushToMainBlockedByPolicy(t *testing.T) {
	resp := handleGitPush(GitPushRequest{
		WorkDir: "/tmp/not-a-real-repo", // will fail at RemoteURL if policy guard is bypassed
		Branch:  "main",
		Force:   false,
	})
	if resp.OK {
		t.Fatal("expected push to main to be blocked regardless of force flag")
	}
	policyErr := "push to main blocked — use a feature branch and PR"
	if resp.Error != policyErr {
		t.Errorf("expected policy error, got: %q", resp.Error)
	}
}

func TestHandleGitPush_ForceWithEmptyBranchReturnsEmptyError(t *testing.T) {
	resp := handleGitPush(GitPushRequest{
		WorkDir: "/tmp/whatever",
		Branch:  "",
		Force:   true,
	})
	if resp.OK {
		t.Fatal("expected OK=false for empty branch")
	}
	if resp.Error != "branch must not be empty" {
		t.Errorf("expected empty-branch error to win over policy, got %q", resp.Error)
	}
}

func TestHTTPGitPush_ForceFlagFlowsThrough(t *testing.T) {
	h := testHandlers(nil)
	var received GitPushRequest
	h.gitPush = func(req GitPushRequest) GitPushResponse {
		received = req
		return GitPushResponse{OK: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(GitPushRequest{WorkDir: "/wd", Branch: "feature/x", Force: true})
	req := httptest.NewRequest(http.MethodPost, "/git/push", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !received.Force {
		t.Error("expected Force=true on handler, got false")
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
