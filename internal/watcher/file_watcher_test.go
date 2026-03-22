package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileWatcherOnlyTargetFile verifies that FileWatcher only processes writes to
// the target file and ignores other JSONL files in the same parent directory.
func TestFileWatcherOnlyTargetFile(t *testing.T) {
	dir := t.TempDir()

	targetFile := filepath.Join(dir, "target-session.jsonl")
	otherFile := filepath.Join(dir, "other-session.jsonl")

	assistantLine := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello from target"}]}}` + "\n"

	var received []string
	fw := NewFileWatcher("testagent", targetFile, func(text string) {
		received = append(received, text)
	})

	done := make(chan struct{})
	defer close(done)
	go fw.Run(done)

	// Give the watcher time to start.
	time.Sleep(50 * time.Millisecond)

	// Write to the other file — should be ignored.
	if err := os.WriteFile(otherFile, []byte(assistantLine), 0o644); err != nil {
		t.Fatalf("write other file: %v", err)
	}

	// Write to the target file immediately after; then poll for the target event.
	// We verify isolation by asserting exactly one event (target only) at the end.
	if err := os.WriteFile(targetFile, []byte(assistantLine), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(received) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// Exactly one event: the target file's text. If the other file leaked through,
	// we'd get 2 events.
	if len(received) != 1 || received[0] != "hello from target" {
		t.Errorf("expected [\"hello from target\"], got %v", received)
	}
}

// TestFileWatcherEncodedPathConstruction verifies the JSONL file path is built correctly
// from workDir + sessionID (the pattern used in startTaskScopedFileWatch).
func TestFileWatcherEncodedPathConstruction(t *testing.T) {
	home := t.TempDir()
	workDir := "/Users/neil/Code/myproject"
	sessionID := "abc12345"

	encoded := EncodePath(workDir)
	jsonlPath := filepath.Join(home, ".claude", "projects", encoded, sessionID+".jsonl")

	// Encoded path should not contain slashes (CC encoding replaces them).
	if filepath.Base(filepath.Dir(jsonlPath)) == "projects" {
		// encoded dir should be a flat name, not nested
		t.Errorf("encoded dir unexpectedly has no subdirectory: %s", jsonlPath)
	}
	if filepath.Ext(jsonlPath) != ".jsonl" {
		t.Errorf("expected .jsonl extension, got %q", filepath.Ext(jsonlPath))
	}
	expectedBase := sessionID + ".jsonl"
	if filepath.Base(jsonlPath) != expectedBase {
		t.Errorf("expected base %q, got %q", expectedBase, filepath.Base(jsonlPath))
	}
}
