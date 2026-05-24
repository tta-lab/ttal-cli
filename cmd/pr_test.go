package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestPRModifyCmd_FlagRegistration(t *testing.T) {
	titleFlag := prModifyCmd.Flag("title")
	if titleFlag == nil {
		t.Fatal("expected --title flag on prModifyCmd")
	}
	prIDFlag := prModifyCmd.Flag("pr-id")
	if prIDFlag == nil {
		t.Fatal("expected --pr-id flag on prModifyCmd")
	}
	bodyFlag := prModifyCmd.Flag("body")
	if bodyFlag != nil {
		t.Error("--body flag should NOT exist on prModifyCmd")
	}
}

func TestPRCreateCmd_NoBodyFlag(t *testing.T) {
	bodyFlag := prCreateCmd.Flag("body")
	if bodyFlag != nil {
		t.Error("--body flag should NOT exist on prCreateCmd")
	}
	prIDFlag := prCreateCmd.Flag("pr-id")
	if prIDFlag != nil {
		t.Error("--pr-id flag should NOT exist on prCreateCmd")
	}
}

func TestPRModifyCmd_HelpReflectsNewContract(t *testing.T) {
	var buf strings.Builder
	prModifyCmd.SetOut(&buf)
	if err := prModifyCmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}
	helpText := buf.String()
	if !strings.Contains(helpText, "stdin") && !strings.Contains(helpText, "heredoc") {
		t.Error("help text should mention stdin or heredoc, got:\n" + helpText)
	}
	if strings.Contains(helpText, "--body") {
		t.Error("help text should NOT mention --body")
	}
}

func TestPRCreateCmd_HelpReflectsNewContract(t *testing.T) {
	var buf strings.Builder
	prCreateCmd.SetOut(&buf)
	if err := prCreateCmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}
	helpText := buf.String()
	if !strings.Contains(helpText, "stdin") && !strings.Contains(helpText, "heredoc") {
		t.Error("help text should mention stdin or heredoc, got:\n" + helpText)
	}
	if strings.Contains(helpText, "--body") {
		t.Error("help text should NOT mention --body")
	}
}

func stubDaemonPRCreate(t *testing.T, fn func(req daemon.PRCreateRequest) (daemon.PRResponse, error)) func() {
	t.Helper()
	orig := daemonPRCreateFn
	daemonPRCreateFn = fn
	return func() { daemonPRCreateFn = orig }
}

func stubDaemonPRModify(t *testing.T, fn func(req daemon.PRModifyRequest) (daemon.PRResponse, error)) func() {
	t.Helper()
	orig := daemonPRModifyFn
	daemonPRModifyFn = fn
	return func() { daemonPRModifyFn = orig }
}

func stubDaemonPRFind(t *testing.T, fn func(req daemon.PRFindRequest) (daemon.PRFindResponse, error)) func() {
	t.Helper()
	orig := daemonPRFindFn
	daemonPRFindFn = fn
	return func() { daemonPRFindFn = orig }
}

func stubPRResolveContext(t *testing.T, fn func() (*pr.Context, error)) func() {
	t.Helper()
	orig := prResolveContextFn
	prResolveContextFn = fn
	return func() { prResolveContextFn = orig }
}

func TestPRModify_TTYStdinNoTitle_LoudError(t *testing.T) {
	// When stdin is a TTY and no --title, should error loudly.
	// TTY detection means readStdinIfPiped returns "".
	// We don't need to stub anything for this test — the early return
	// hits the error before ResolveContext is called.
	var buf strings.Builder
	prModifyCmd.SetOut(&buf)
	prModifyCmd.SetErr(&buf)
	err := prModifyCmd.RunE(prModifyCmd, nil)
	if err == nil {
		t.Fatal("expected error when stdin is TTY and no --title")
	}
	if !strings.Contains(err.Error(), "nothing to update") {
		t.Errorf("error should mention 'nothing to update', got: %v", err)
	}
	// Should NOT mention context resolution errors
	if strings.Contains(err.Error(), "worktree") || strings.Contains(err.Error(), "task") {
		t.Errorf("error should be about missing input, not context resolution: %v", err)
	}
}

func TestPRModify_PipedBody(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r

	if _, err := w.WriteString("new body content\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	prCalled := false
	defer stubPRResolveContext(t, func() (*pr.Context, error) {
		prCalled = true
		return &pr.Context{
			Task:  &taskwarrior.Task{PRID: "42"},
			Owner: "owner",
			Repo:  "repo",
			Info:  &gitprovider.RepoInfo{Owner: "owner", Repo: "repo"},
		}, nil
	})()

	modifyCalled := false
	defer stubDaemonPRModify(t, func(req daemon.PRModifyRequest) (daemon.PRResponse, error) {
		modifyCalled = true
		if req.Body != "new body content" {
			t.Errorf("Body = %q, want %q", req.Body, "new body content")
		}
		if req.Index != 42 {
			t.Errorf("Index = %d, want %d", req.Index, 42)
		}
		return daemon.PRResponse{OK: true, PRIndex: 42, PRURL: "https://pr/42"}, nil
	})()

	prModifyCmd.SetArgs([]string{})
	err = prModifyCmd.RunE(prModifyCmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !prCalled {
		t.Error("ResolveContext should be called")
	}
	if !modifyCalled {
		t.Error("daemon PRModify should be called")
	}
}

func TestPRModify_ResolvesPRFromCurrentBranchWhenPRIDMissing(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r

	if _, err := w.WriteString("new body content\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	defer stubPRResolveContext(t, func() (*pr.Context, error) {
		return &pr.Context{
			Task:  &taskwarrior.Task{UUID: "abc12345-0000-0000-0000-000000000000", Project: "ttal"},
			Owner: "owner",
			Repo:  "repo",
			Info: &gitprovider.RepoInfo{
				Owner:         "owner",
				Repo:          "repo",
				Provider:      gitprovider.ProviderForgejo,
				Host:          "git.example.test",
				DefaultBranch: "main",
			},
			Alias: "ttal",
		}, nil
	})()

	defer stubCurrentBranch(t, func(uuid, alias, workDir string) string {
		if uuid != "abc12345-0000-0000-0000-000000000000" {
			t.Errorf("uuid = %q", uuid)
		}
		if alias != "ttal" {
			t.Errorf("alias = %q", alias)
		}
		return "feat/current-work"
	})()

	findCalled := false
	defer stubDaemonPRFind(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		findCalled = true
		if req.ProviderType != "forgejo" {
			t.Errorf("ProviderType = %q, want forgejo", req.ProviderType)
		}
		if req.Host != "git.example.test" {
			t.Errorf("Host = %q", req.Host)
		}
		if req.Owner != "owner" || req.Repo != "repo" {
			t.Errorf("repo = %s/%s, want owner/repo", req.Owner, req.Repo)
		}
		if req.Head != "feat/current-work" {
			t.Errorf("Head = %q", req.Head)
		}
		if req.Base != "main" {
			t.Errorf("Base = %q", req.Base)
		}
		if req.ProjectAlias != "ttal" {
			t.Errorf("ProjectAlias = %q", req.ProjectAlias)
		}
		return daemon.PRFindResponse{OK: true, PRIndex: 24, PRURL: "https://pr/24"}, nil
	})()

	modifyCalled := false
	defer stubDaemonPRModify(t, func(req daemon.PRModifyRequest) (daemon.PRResponse, error) {
		modifyCalled = true
		if req.Index != 24 {
			t.Errorf("Index = %d, want 24", req.Index)
		}
		return daemon.PRResponse{OK: true, PRIndex: 24, PRURL: "https://pr/24"}, nil
	})()

	prModifyCmd.SetArgs([]string{})
	if err := prModifyCmd.RunE(prModifyCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !findCalled {
		t.Error("daemon PRFind should be called")
	}
	if !modifyCalled {
		t.Error("daemon PRModify should be called")
	}
}

func TestPRCreate_PipedBody(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r

	if _, err := w.WriteString("multi-line\nbody content\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	defer stubCurrentBranch(t, func(uuid, alias, workDir string) string {
		return "test-branch"
	})()

	defer stubGitPush(t, func(req daemon.GitPushRequest) (daemon.GitPushResponse, error) {
		return daemon.GitPushResponse{OK: true}, nil
	})()

	resolveCalled := false
	defer stubPRResolveContext(t, func() (*pr.Context, error) {
		resolveCalled = true
		return &pr.Context{
			Task:  &taskwarrior.Task{PRID: "42", UUID: "abc12345-0000-0000-0000-000000000000"},
			Owner: "owner",
			Repo:  "repo",
			Info:  &gitprovider.RepoInfo{Owner: "owner", Repo: "repo", DefaultBranch: "main"},
		}, nil
	})()

	createCalled := false
	defer stubDaemonPRCreate(t, func(req daemon.PRCreateRequest) (daemon.PRResponse, error) {
		createCalled = true
		if req.Body != "multi-line\nbody content" {
			t.Errorf("Body = %q, want %q", req.Body, "multi-line\nbody content")
		}
		if req.Title != "feat: test" {
			t.Errorf("Title = %q, want %q", req.Title, "feat: test")
		}
		return daemon.PRResponse{OK: true, PRIndex: 99, PRURL: "https://pr/99"}, nil
	})()

	defer stubSetPRID(t, func(uuid, prID string) error {
		return nil
	})()

	defer stubDaemonNotify(t, func(req daemon.NotifyRequest) error {
		return nil
	})()

	defer stubDaemonSend(t, func(req daemon.SendRequest) error {
		return nil
	})()

	err = prCreateCmd.RunE(prCreateCmd, []string{"feat: test"})
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !resolveCalled {
		t.Error("ResolveContext should be called")
	}
	if !createCalled {
		t.Error("daemon PRCreate should be called")
	}
}

func stubCurrentBranch(t *testing.T, fn func(uuid, alias, workDir string) string) func() {
	t.Helper()
	orig := currentBranchFn
	currentBranchFn = fn
	return func() { currentBranchFn = orig }
}

func stubGitPush(t *testing.T, fn func(req daemon.GitPushRequest) (daemon.GitPushResponse, error)) func() {
	t.Helper()
	orig := gitPushFn
	gitPushFn = fn
	return func() { gitPushFn = orig }
}

func stubSetPRID(t *testing.T, fn func(uuid, prID string) error) func() {
	t.Helper()
	orig := setPRIDFn
	setPRIDFn = fn
	return func() { setPRIDFn = orig }
}

func stubDaemonNotify(t *testing.T, fn func(req daemon.NotifyRequest) error) func() {
	t.Helper()
	orig := daemonNotifyFn
	daemonNotifyFn = fn
	return func() { daemonNotifyFn = orig }
}

func stubDaemonSend(t *testing.T, fn func(req daemon.SendRequest) error) func() {
	t.Helper()
	orig := daemonSendFn
	daemonSendFn = fn
	return func() { daemonSendFn = orig }
}
