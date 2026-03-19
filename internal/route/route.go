package route

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
)

const routingDir = "routing"

// Request is written to ~/.ttal/routing/<agent>.json by the router.
// The daemon checks for this file during handleBreathe to compose
// the restart with routing context.
type Request struct {
	TaskUUID    string    `json:"task_uuid"`
	RolePrompt  string    `json:"role_prompt"`
	Trigger     string    `json:"trigger"`
	ProjectPath string    `json:"project_path,omitempty"`
	RoutedBy    string    `json:"routed_by,omitempty"`
	Message     string    `json:"message,omitempty"`
	Team        string    `json:"team,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Stage writes a routing request file for the daemon to consume.
// If a routing file already exists (agent hasn't breathed yet), it is
// overwritten and a warning is logged — latest route wins.
func Stage(agentName string, req Request) error {
	dir := filepath.Join(config.DefaultDataDir(), routingDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create routing dir: %w", err)
	}
	path := filepath.Join(dir, agentName+".json")
	if _, err := os.Stat(path); err == nil {
		log.Printf("[route] warning: overwriting unconsumed routing file for %s (agent hasn't breathed yet)", agentName)
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Consume reads and deletes a routing request file. Returns nil, nil if no file exists.
func Consume(agentName string) (*Request, error) {
	path := filepath.Join(config.DefaultDataDir(), routingDir, agentName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read routing file: %w", err)
	}
	os.Remove(path) // consumed
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse routing file: %w", err)
	}
	return &req, nil
}
