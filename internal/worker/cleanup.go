package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

const cleanupDir = "cleanup"

// CleanupRequest is written to ~/.ttal/cleanup/<session>.json by workers after merge.
type CleanupRequest struct {
	SessionID string    `json:"session_id"`
	TaskUUID  string    `json:"task_uuid"`
	Team      string    `json:"team,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// RequestCleanup writes a cleanup request file for the daemon to process.
// This is fire-and-forget — the file persists even if the daemon is down.
// All teams share a single cleanup dir (~/.ttal/cleanup/) — requests are globally unique.
func RequestCleanup(sessionID, taskUUID string) error {
	dir := filepath.Join(config.DefaultDataDir(), cleanupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create cleanup dir: %w", err)
	}

	req := CleanupRequest{
		SessionID: sessionID,
		TaskUUID:  taskUUID,
		Team:      os.Getenv("TTAL_TEAM"),
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, sessionID+".json")
	return os.WriteFile(path, data, 0o644)
}

// CleanupDir returns the path to ~/.ttal/cleanup/ (shared across all teams).
func CleanupDir() (string, error) {
	return filepath.Join(config.DefaultDataDir(), cleanupDir), nil
}

// RunCleanup processes a single cleanup request file: sets the team env,
// closes the worker (which marks the task done), and removes the request file.
// When SessionID is empty, MarkDone is called directly since Close is skipped.
// Designed for manual invocation via `ttal worker cleanup`.
func RunCleanup(path string, force bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	var req CleanupRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	// Set TTAL_TEAM so taskwarrior commands resolve the correct taskrc.
	if req.Team != "" {
		prev := os.Getenv("TTAL_TEAM")
		_ = os.Setenv("TTAL_TEAM", req.Team)
		defer func() { _ = os.Setenv("TTAL_TEAM", prev) }()
	}

	fmt.Printf("Processing cleanup: session=%s task=%s team=%s\n", req.SessionID, req.TaskUUID, req.Team)

	if req.SessionID == "" {
		if req.TaskUUID != "" {
			if err := taskwarrior.MarkDone(req.TaskUUID); err != nil {
				return fmt.Errorf("failed to mark task done %s: %w", req.TaskUUID, err)
			}
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove request file: %w", err)
		}
		return nil
	}

	_, closeErr := Close(req.SessionID, force)
	if closeErr != nil {
		return fmt.Errorf("close failed for %s: %w", req.SessionID, closeErr)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove request file: %w", err)
	}

	fmt.Printf("Cleanup completed: session=%s\n", req.SessionID)
	return nil
}

// RunPendingCleanups processes all .json files in the cleanup directory.
func RunPendingCleanups(force bool) error {
	dir, err := CleanupDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No pending cleanup requests")
			return nil
		}
		return fmt.Errorf("failed to read cleanup dir: %w", err)
	}

	// Filter to only .json files
	var jsonFiles []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			jsonFiles = append(jsonFiles, e)
		}
	}

	if len(jsonFiles) == 0 {
		fmt.Println("No pending cleanup requests")
		return nil
	}

	var count int
	for _, e := range jsonFiles {
		if err := RunCleanup(filepath.Join(dir, e.Name()), force); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", e.Name(), err)
			continue
		}
		count++
	}

	fmt.Printf("Processed %d cleanup request(s)\n", count)
	return nil
}
