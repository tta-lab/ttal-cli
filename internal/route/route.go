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
// Consumed by the CC SessionStart hook (ttal context) which injects RolePrompt
// and Message as the session system message.
type Request struct {
	TaskUUID   string `json:"task_uuid"`
	RolePrompt string `json:"role_prompt"`
	// Trigger is no longer used: context injection via the CC SessionStart hook
	// means Message alone is sufficient to kick the agent into action at startup.
	// Kept for backward compatibility with existing staged route files.
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
// Parse happens before delete: if JSON is malformed the file is left on disk for
// diagnosis and the error is returned. The file is only removed on successful parse.
func Consume(agentName string) (*Request, error) {
	path := filepath.Join(config.DefaultDataDir(), routingDir, agentName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read routing file: %w", err)
	}
	// Parse first — if the file is corrupt, leave it on disk for diagnosis.
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse routing file: %w", err)
	}
	// Only remove after successful parse to prevent assignment loss on corrupt write.
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("remove routing file (would cause re-delivery): %w", err)
	}
	return &req, nil
}
