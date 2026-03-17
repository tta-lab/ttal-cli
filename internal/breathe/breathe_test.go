package breathe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSyntheticSession(t *testing.T) {
	dir := t.TempDir()

	cfg := SessionConfig{
		CWD:       "/Users/neil/Code/ttal-cli",
		CCVersion: "2.1.47",
		GitBranch: "main",
		Handoff:   "# Handoff\n\n## What was done\n- Feature X",
	}

	sessionID, err := WriteSyntheticSession(dir, cfg)
	if err != nil {
		t.Fatalf("WriteSyntheticSession: %v", err)
	}

	if len(sessionID) != 36 {
		t.Errorf("session ID length = %d, want 36", len(sessionID))
	}

	data, err := os.ReadFile(filepath.Join(dir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("read JSONL: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Line 1: file-history-snapshot
	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if snapshot["type"] != "file-history-snapshot" {
		t.Errorf("line 1 type = %v, want file-history-snapshot", snapshot["type"])
	}

	// Line 2: user message with handoff
	var userMsg map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &userMsg); err != nil {
		t.Fatalf("parse user msg: %v", err)
	}
	if userMsg["type"] != "user" {
		t.Errorf("line 2 type = %v, want user", userMsg["type"])
	}
	if userMsg["sessionId"] != sessionID {
		t.Errorf("sessionId mismatch")
	}
	if userMsg["cwd"] != cfg.CWD {
		t.Errorf("cwd mismatch")
	}
	msg := userMsg["message"].(map[string]interface{})
	if msg["content"] != cfg.Handoff {
		t.Errorf("content mismatch")
	}
	if userMsg["permissionMode"] != "bypassPermissions" {
		t.Errorf("permissionMode = %v, want bypassPermissions", userMsg["permissionMode"])
	}
}
