package status

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AgentStatus represents the live state of an agent's CC session.
type AgentStatus struct {
	Agent               string    `json:"agent"`
	ContextUsedPct      float64   `json:"context_used_pct"`
	ContextRemainingPct float64   `json:"context_remaining_pct"`
	ModelID             string    `json:"model_id"`
	ModelName           string    `json:"model_name"`
	SessionID           string    `json:"session_id"`
	CCVersion           string    `json:"cc_version"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// StatusDir returns the path to the status directory.
func StatusDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ttal", "status")
}

// ReadAgent reads the status file for a single agent.
// Returns nil if no status file exists (agent not running or no data yet).
func ReadAgent(name string) (*AgentStatus, error) {
	return readAgentFrom(StatusDir(), name)
}

// ReadAll reads status files for all agents that have them.
func ReadAll() ([]AgentStatus, error) {
	return readAllFrom(StatusDir())
}

// Remove deletes the status file for an agent (called on session teardown).
func Remove(name string) error {
	return removeFrom(StatusDir(), name)
}

// IsStale returns true if the status hasn't been updated in the given duration.
func (s *AgentStatus) IsStale(threshold time.Duration) bool {
	return time.Since(s.UpdatedAt) > threshold
}

// readAgentFrom reads the status file for a single agent from the given directory.
func readAgentFrom(dir, name string) (*AgentStatus, error) {
	path := filepath.Join(dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read status for %s: %w", name, err)
	}

	var s AgentStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse status for %s: %w", name, err)
	}
	return &s, nil
}

// readAllFrom reads status files for all agents from the given directory.
func readAllFrom(dir string) ([]AgentStatus, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read status dir: %w", err)
	}

	statuses := make([]AgentStatus, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // strip .json
		if len(name) == 0 || name[0] == '.' {
			continue // skip empty names and .tmp files
		}
		s, err := readAgentFrom(dir, name)
		if err != nil || s == nil {
			continue
		}
		statuses = append(statuses, *s)
	}
	return statuses, nil
}

// removeFrom deletes the status file for an agent from the given directory.
func removeFrom(dir, name string) error {
	path := filepath.Join(dir, name+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
