package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

const cleanupDir = "cleanup"

// CleanupRequest is written to ~/.ttal/cleanup/<session>.json by workers after merge.
type CleanupRequest struct {
	SessionID string    `json:"session_id"`
	TaskUUID  string    `json:"task_uuid"`
	CreatedAt time.Time `json:"created_at"`
}

// RequestCleanup writes a cleanup request file for the daemon to process.
// This is fire-and-forget — the file persists even if the daemon is down.
func RequestCleanup(sessionID, taskUUID string) error {
	dir := filepath.Join(config.ResolveDataDir(), cleanupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create cleanup dir: %w", err)
	}

	req := CleanupRequest{
		SessionID: sessionID,
		TaskUUID:  taskUUID,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, sessionID+".json")
	return os.WriteFile(path, data, 0o644)
}

// CleanupDir returns the path to <data_dir>/cleanup/.
func CleanupDir() (string, error) {
	return filepath.Join(config.ResolveDataDir(), cleanupDir), nil
}
