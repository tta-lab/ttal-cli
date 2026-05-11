package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecuteCleanup_LegacySessionID(t *testing.T) {
	origClose := closeFn
	var capturedID string
	closeFn = func(sessionID string, force bool) (*CloseResult, error) {
		capturedID = sessionID
		return &CloseResult{Cleaned: true}, nil
	}
	t.Cleanup(func() { closeFn = origClose })

	req := CleanupRequest{
		SessionID: "w-abc12345-fix-auth",
		TaskUUID:  "",
		CreatedAt: time.Now(),
	}
	path := filepath.Join(t.TempDir(), "test.json")
	_ = os.WriteFile(path, []byte{}, 0o644)

	if err := ExecuteCleanup(req, path, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "w-abc12345-fix-auth" {
		t.Errorf("Close called with %q, want %q", capturedID, "w-abc12345-fix-auth")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected request file to be removed")
	}
}

func TestExecuteCleanup_NewTaskUUID(t *testing.T) {
	origClose := closeFn
	var capturedID string
	closeFn = func(sessionID string, force bool) (*CloseResult, error) {
		capturedID = sessionID
		return &CloseResult{Cleaned: true}, nil
	}
	t.Cleanup(func() { closeFn = origClose })

	req := CleanupRequest{
		SessionID: "",
		TaskUUID:  "e9d4b7c1-1234-5678-9abc-def012345678",
		CreatedAt: time.Now(),
	}
	path := filepath.Join(t.TempDir(), "test.json")
	_ = os.WriteFile(path, []byte{}, 0o644)

	if err := ExecuteCleanup(req, path, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "e9d4b7c1" {
		t.Errorf("Close called with hex %q, want %q", capturedID, "e9d4b7c1")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected request file to be removed")
	}
}

func TestExecuteCleanup_BothIDsPrefersTaskUUID(t *testing.T) {
	origClose := closeFn
	var capturedID string
	closeFn = func(sessionID string, force bool) (*CloseResult, error) {
		capturedID = sessionID
		return &CloseResult{Cleaned: true}, nil
	}
	t.Cleanup(func() { closeFn = origClose })

	req := CleanupRequest{
		SessionID: "w-legacy456-fix-auth",
		TaskUUID:  "abc12345-0000-0000-0000-000000000000",
		CreatedAt: time.Now(),
	}
	path := filepath.Join(t.TempDir(), "test.json")
	_ = os.WriteFile(path, []byte{}, 0o644)

	if err := ExecuteCleanup(req, path, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "abc12345" {
		t.Errorf("Close should use TaskUUID, called with %q, want %q", capturedID, "abc12345")
	}
}

func TestExecuteCleanup_NeitherSet(t *testing.T) {
	origClose := closeFn
	var closeCalled bool
	closeFn = func(sessionID string, force bool) (*CloseResult, error) {
		closeCalled = true
		return &CloseResult{Cleaned: true}, nil
	}
	t.Cleanup(func() { closeFn = origClose })

	req := CleanupRequest{
		SessionID: "",
		TaskUUID:  "",
		CreatedAt: time.Now(),
	}
	path := filepath.Join(t.TempDir(), "test.json")
	_ = os.WriteFile(path, []byte{}, 0o644)

	if err := ExecuteCleanup(req, path, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closeCalled {
		t.Error("Close should not be called when neither ID is set")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected request file to be removed")
	}
}
