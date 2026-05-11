package worker

import (
	"testing"
)

func TestEnsureWindowAvailable_NoSessionError(t *testing.T) {
	origSessionExists := tmuxSessionExistsFn
	t.Cleanup(func() { tmuxSessionExistsFn = origSessionExists })

	tmuxSessionExistsFn = func(name string) bool { return false }

	cfg := SpawnConfig{Name: "test-worker", Force: false}
	target := TmuxTarget{Session: "ttal-default-astra", Window: "coder"}

	err := ensureWindowAvailable(cfg, target)
	if err == nil {
		t.Fatal("expected error for missing owner session")
	}
}

func TestEnsureWindowAvailable_WindowExistsNoForce(t *testing.T) {
	origSessionExists := tmuxSessionExistsFn
	origWindowExists := tmuxWindowExistsFn
	t.Cleanup(func() {
		tmuxSessionExistsFn = origSessionExists
		tmuxWindowExistsFn = origWindowExists
	})

	tmuxSessionExistsFn = func(name string) bool { return true }
	tmuxWindowExistsFn = func(session, window string) bool { return true }

	cfg := SpawnConfig{Name: "test-worker", Force: false}
	target := TmuxTarget{Session: "ttal-default-astra", Window: "coder"}

	err := ensureWindowAvailable(cfg, target)
	if err == nil {
		t.Fatal("expected conflict error for existing window")
	}
}
