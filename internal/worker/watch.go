package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

const watchDir = "watch"

// WatchRequest is written to ~/.ttal/watch/<sessionID>.json by workers after PR creation.
// The daemon picks these up via fsnotify and starts polling CI status.
type WatchRequest struct {
	SessionID   string    `json:"session_id"`
	TaskUUID    string    `json:"task_uuid"`
	Team        string    `json:"team,omitempty"`
	Owner       string    `json:"owner"`
	Repo        string    `json:"repo"`
	PRIndex     int64     `json:"pr_index"`
	Branch      string    `json:"branch"`
	Provider    string    `json:"provider"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// RequestWatch writes a watch request file for the daemon to process.
// Fire-and-forget — the file persists even if the daemon is down.
func RequestWatch(req WatchRequest) error {
	dir := filepath.Join(config.DefaultDataDir(), watchDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create watch dir: %w", err)
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, req.SessionID+".json")
	return os.WriteFile(path, data, 0o644)
}

// WatchDir returns the path to ~/.ttal/watch/.
func WatchDir() string {
	return filepath.Join(config.DefaultDataDir(), watchDir)
}
