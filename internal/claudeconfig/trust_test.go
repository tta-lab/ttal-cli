package claudeconfig_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/claudeconfig"
)

func TestUpsertTrust_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	added, err := claudeconfig.UpsertTrust(path, []string{"/workspace/agent1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 1 {
		t.Fatalf("expected 1 added, got %d", added)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	projects, _ := raw["projects"].(map[string]any)
	if projects == nil {
		t.Fatal("missing projects key")
	}
	entry, ok := projects["/workspace/agent1"]
	if !ok {
		t.Fatal("missing trust entry for /workspace/agent1")
	}
	m, _ := entry.(map[string]any)
	if m["hasTrustDialogAccepted"] != true {
		t.Error("hasTrustDialogAccepted not set")
	}
}

func TestUpsertTrust_FileExistsWithoutEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	existing := map[string]any{
		"hasCompletedOnboarding": true,
		"projects": map[string]any{
			"/workspace/other": map[string]any{
				"hasTrustDialogAccepted": true,
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0o644)

	added, err := claudeconfig.UpsertTrust(path, []string{"/workspace/newagent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 1 {
		t.Fatalf("expected 1 added, got %d", added)
	}

	data, _ = os.ReadFile(path)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	projects, _ := raw["projects"].(map[string]any)

	// Existing entry must be preserved
	if _, ok := projects["/workspace/other"]; !ok {
		t.Error("existing entry was removed")
	}
	// New entry must be present
	if _, ok := projects["/workspace/newagent"]; !ok {
		t.Error("new entry not added")
	}
}

func TestUpsertTrust_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	// First call
	_, _ = claudeconfig.UpsertTrust(path, []string{"/workspace/agent1"})

	// Second call — same path
	added, err := claudeconfig.UpsertTrust(path, []string{"/workspace/agent1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 0 {
		t.Fatalf("expected 0 added (idempotent), got %d", added)
	}
}

func TestUpsertTrust_MultiplePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	paths := []string{"/workspace/a", "/workspace/b", "/workspace/c"}
	added, err := claudeconfig.UpsertTrust(path, paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 3 {
		t.Fatalf("expected 3 added, got %d", added)
	}

	data, _ := os.ReadFile(path)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	projects, _ := raw["projects"].(map[string]any)
	for _, p := range paths {
		if _, ok := projects[p]; !ok {
			t.Errorf("missing entry for %s", p)
		}
	}
}
