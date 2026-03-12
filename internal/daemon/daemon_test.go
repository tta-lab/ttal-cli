package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/watcher"
)

func TestSessionJSONLExists(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	agentPath := "/tmp/ttal-test-agent-sessionjsonl"
	encoded := watcher.EncodePath(agentPath)
	dir := filepath.Join(home, ".claude", "projects", encoded)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("cannot create test project dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	t.Run("returns false when JSONL missing", func(t *testing.T) {
		if sessionJSONLExists("nonexistent-session-id", agentPath) {
			t.Error("expected false for missing JSONL, got true")
		}
	})

	t.Run("returns true when JSONL present", func(t *testing.T) {
		sessionID := "test-session-12345678"
		jsonlPath := filepath.Join(dir, sessionID+".jsonl")
		if err := os.WriteFile(jsonlPath, []byte(`{"type":"test"}`+"\n"), 0o644); err != nil {
			t.Fatalf("cannot create test JSONL: %v", err)
		}
		t.Cleanup(func() { os.Remove(jsonlPath) })

		if !sessionJSONLExists(sessionID, agentPath) {
			t.Error("expected true for existing JSONL, got false")
		}
	})
}
