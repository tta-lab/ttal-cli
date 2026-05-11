//nolint:goconst // test clarity prefers inline strings over constants
package worker

import (
	"testing"
)

func TestCleanupWorker_KillsWindowNotSession(t *testing.T) {
	origWindowExists := tmuxWindowExistsFn
	origKillWindow := tmuxKillWindowFn
	origRemoveWorktree := removeWorktreeFn
	t.Cleanup(func() {
		tmuxWindowExistsFn = origWindowExists
		tmuxKillWindowFn = origKillWindow
		removeWorktreeFn = origRemoveWorktree
	})
	removeWorktreeFn = func(gitRoot, workDir, branch string) error { return nil }

	var killedSession, killedWindow string
	tmuxWindowExistsFn = func(session, window string) bool { return true }
	tmuxKillWindowFn = func(session, window string) error {
		killedSession = session
		killedWindow = window
		return nil
	}

	target := TmuxTarget{Session: "ttal-default-astra", Window: "coder"}
	if err := cleanupWorker(target, "/tmp/wt", "branch", "/tmp/gitroot"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if killedSession != "ttal-default-astra" {
		t.Errorf("killed session = %q, want %q", killedSession, "ttal-default-astra")
	}
	if killedWindow != CoderAgentName {
		t.Errorf("killed window = %q, want %q", killedWindow, CoderAgentName)
	}
}

func TestCleanupWorker_NoWindowExists(t *testing.T) {
	origWindowExists := tmuxWindowExistsFn
	origKillWindow := tmuxKillWindowFn
	origRemoveWorktree := removeWorktreeFn
	t.Cleanup(func() {
		tmuxWindowExistsFn = origWindowExists
		tmuxKillWindowFn = origKillWindow
		removeWorktreeFn = origRemoveWorktree
	})
	removeWorktreeFn = func(gitRoot, workDir, branch string) error { return nil }

	var killCalled bool
	tmuxWindowExistsFn = func(session, window string) bool { return false }
	tmuxKillWindowFn = func(session, window string) error {
		killCalled = true
		return nil
	}

	target := TmuxTarget{Session: "ttal-default-astra", Window: "coder"}
	if err := cleanupWorker(target, "/tmp/wt", "branch", "/tmp/gitroot"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if killCalled {
		t.Error("KillWindow should not be called when window doesn't exist")
	}
}
