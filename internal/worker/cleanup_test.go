package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRequestCleanup_TaskBasedFilename(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, cleanupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessionID := "w-abc12345-fix-auth"
	taskUUID := "abc12345-0000-0000-0000-000000000000"

	req := CleanupRequest{
		SessionID: sessionID,
		TaskUUID:  taskUUID,
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(req)
	path := filepath.Join(dir, taskUUID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected task-based cleanup file at %s", path)
	}

	oldPath := filepath.Join(dir, sessionID+".json")
	if _, err := os.Stat(oldPath); err == nil {
		t.Errorf("old session-based file %s should not exist when taskUUID is set", oldPath)
	}
}

func TestRequestCleanup_SessionFallbackFilename(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, cleanupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessionID := "w-abc12345-fix-auth"
	taskUUID := ""

	req := CleanupRequest{
		SessionID: sessionID,
		TaskUUID:  taskUUID,
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(req)
	path := filepath.Join(dir, sessionID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected session-based cleanup file at %s", path)
	}
}

func TestExecuteCleanup_NoSessionNoTaskUUID(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	req := CleanupRequest{
		SessionID: "",
		TaskUUID:  "",
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(req)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ExecuteCleanup(req, path, false); err != nil {
		t.Errorf("expected no error for empty request, got: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected cleanup file to be removed after ExecuteCleanup")
	}
}

func TestRunCleanup_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunCleanup(path, false)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestCleanupRequest_MixedFormatFiles(t *testing.T) {
	tmpDir := t.TempDir()

	sessionID := "w-legacy123-fix-auth"
	legacyPath := filepath.Join(tmpDir, sessionID+".json")
	legacyReq := CleanupRequest{
		SessionID: sessionID,
		TaskUUID:  "legacy123-fix-auth",
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(legacyReq)
	if err := os.WriteFile(legacyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	taskUUID := "new45678-0000-0000-0000-000000000000"
	newPath := filepath.Join(tmpDir, taskUUID+".json")
	newReq := CleanupRequest{
		SessionID: "w-new45678-fix-auth",
		TaskUUID:  taskUUID,
		CreatedAt: time.Now(),
	}
	data, _ = json.Marshal(newReq)
	if err := os.WriteFile(newPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		t.Errorf("legacy session-based file not found: %s", legacyPath)
	}
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("new task-based file not found: %s", newPath)
	}
}

func TestExecuteCleanup_EmptyRequestRemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	req := CleanupRequest{
		SessionID: "",
		TaskUUID:  "",
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(req)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ExecuteCleanup(req, path, false); err != nil {
		t.Errorf("expected no error for empty request, got: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected cleanup file to be removed after ExecuteCleanup")
	}
}
