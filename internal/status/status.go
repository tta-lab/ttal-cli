package status

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
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

// StatusDir returns the consolidated status directory (~/.ttal/status/).
func StatusDir() string {
	return filepath.Join(config.DefaultDataDir(), "status")
}

// ReadAgent reads the status file for a single agent in the given team.
// Returns nil if no status file exists (agent not running or no data yet).
func ReadAgent(team, name string) (*AgentStatus, error) {
	return readAgentFrom(StatusDir(), team, name)
}

// ReadAll reads status files for all agents in the given team.
func ReadAll(team string) ([]AgentStatus, error) {
	return readAllFrom(StatusDir(), team)
}

// WriteAgent atomically writes an agent's status file with team prefix.
func WriteAgent(team string, s AgentStatus) error {
	return writeAgentTo(StatusDir(), team, s)
}

// IsStale returns true if the status hasn't been updated in the given duration.
func (s *AgentStatus) IsStale(threshold time.Duration) bool {
	return time.Since(s.UpdatedAt) > threshold
}

// statusFileName returns the team-prefixed filename for an agent: {team}-{agent}.
func statusFileName(team, agent string) string {
	return team + "-" + agent
}

// readAgentFrom reads the status file for a single agent from the given directory.
func readAgentFrom(dir, team, name string) (*AgentStatus, error) {
	return readAgentFromFile(dir, statusFileName(team, name)+".json")
}

// readAllFrom reads status files for all agents matching the team prefix.
func readAllFrom(dir, team string) ([]AgentStatus, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read status dir: %w", err)
	}

	prefix := team + "-"
	statuses := make([]AgentStatus, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // strip .json
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		s, err := readAgentFromFile(dir, entry.Name())
		if err != nil {
			log.Printf("[status] skipping corrupt status file %s: %v", entry.Name(), err)
			continue
		}
		if s == nil {
			continue
		}
		statuses = append(statuses, *s)
	}
	return statuses, nil
}

// readAgentFromFile reads a status file by its full filename (without .json extension handling).
func readAgentFromFile(dir, filename string) (*AgentStatus, error) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read status file %s: %w", filename, err)
	}

	var s AgentStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse status file %s: %w", filename, err)
	}
	return &s, nil
}

// writeAgentTo atomically writes an agent's status to the given directory.
func writeAgentTo(dir, team string, s AgentStatus) error {
	if s.Agent == "" || strings.ContainsAny(s.Agent, "/\\") || s.Agent[0] == '.' {
		return fmt.Errorf("invalid agent name: %q", s.Agent)
	}
	if team == "" || strings.ContainsAny(team, "/\\") || team[0] == '.' {
		return fmt.Errorf("invalid team name: %q", team)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create status dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	data = append(data, '\n')

	base := statusFileName(team, s.Agent)
	tmp := filepath.Join(dir, "."+base+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}

	target := filepath.Join(dir, base+".json")
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
