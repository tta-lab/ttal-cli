package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cleanupDir = "cleanup"

// CleanupRequest is written to ~/.ttal/cleanup/<session>.json by workers after merge.
type CleanupRequest struct {
	SessionName string    `json:"session_name"`
	TaskUUID    string    `json:"task_uuid"`
	CreatedAt   time.Time `json:"created_at"`
}

// RequestCleanup writes a cleanup request file for the daemon to process.
// This is fire-and-forget — the file persists even if the daemon is down.
func RequestCleanup(sessionName, taskUUID string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".ttal", cleanupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create cleanup dir: %w", err)
	}

	req := CleanupRequest{
		SessionName: sessionName,
		TaskUUID:    taskUUID,
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, sessionName+".json")
	return os.WriteFile(path, data, 0o644)
}

// CleanupDir returns the path to ~/.ttal/cleanup/.
func CleanupDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", cleanupDir), nil
}
