package route

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageAndConsume(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".ttal"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Stage("inke", Request{
		TaskUUID: "abc12345", RolePrompt: "Design this", Trigger: "New task",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := Consume("inke")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected request, got nil")
	}
	if got.TaskUUID != "abc12345" {
		t.Errorf("got TaskUUID %q, want %q", got.TaskUUID, "abc12345")
	}

	// Second consume returns nil (file already deleted)
	got2, err := Consume("inke")
	if err != nil {
		t.Fatal(err)
	}
	if got2 != nil {
		t.Error("expected nil on second consume, got non-nil")
	}
}

func TestConsumeNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	got, err := Consume("nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil when no file exists")
	}
}

func TestStageOverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".ttal"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Stage("inke", Request{TaskUUID: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := Stage("inke", Request{TaskUUID: "second"}); err != nil {
		t.Fatal(err)
	}

	got, err := Consume("inke")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected request")
	}
	if got.TaskUUID != "second" {
		t.Errorf("expected second to win, got %q", got.TaskUUID)
	}
}
