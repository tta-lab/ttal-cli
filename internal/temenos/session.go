package temenos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const defaultMCPPort = 9783

// mcpServerEntry holds the typed fields for one MCP server config entry.
type mcpServerEntry struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// mcpConfigDoc is the top-level MCP config document.
type mcpConfigDoc struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// MCPConfig returns the inline JSON for the temenos MCP server configuration.
// Uses encoding/json so special characters in token are safely escaped.
// Pure function — no side effects.
func MCPConfig(port int, token string) string {
	doc := mcpConfigDoc{
		MCPServers: map[string]mcpServerEntry{
			"temenos": {
				Type: "http",
				URL:  fmt.Sprintf("http://127.0.0.1:%d", port),
				Headers: map[string]string{
					"X-Session-Token": token,
				},
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		// json.Marshal on a plain struct with string values never errors;
		// this branch exists for defensive completeness.
		return ""
	}
	return string(data)
}

// RegisterSessionForAgent registers a temenos session for a CC worker or manager.
// writePaths are paths the agent may write to (e.g. worktree dir, git common dir).
// excludeReadPath, if non-empty, is excluded from the read_paths list passed to temenos
// (typically the worker's own worktree root, which is covered by writePaths).
//
// Returns the MCP config JSON, the session token, and any error.
func RegisterSessionForAgent(
	ctx context.Context, agent string, writePaths []string, excludeReadPath string,
) (mcpJSON, token string, err error) {
	readPaths := gatherReadPaths(excludeReadPath)

	c := New("")
	token, err = c.RegisterSession(ctx, agent, writePaths, readPaths)
	if err != nil {
		return "", "", fmt.Errorf("temenos: register session for %s: %w", agent, err)
	}

	return MCPConfig(defaultMCPPort, token), token, nil
}

// gatherReadPaths returns all active project paths, excluding excludePath if non-empty.
// Non-fatal: on store errors, returns an empty slice and logs nothing (temenos baseline covers common paths).
func gatherReadPaths(excludePath string) []string {
	store := project.NewStore(config.ResolveProjectsPath())
	projects, err := store.List(false)
	if err != nil {
		return nil
	}
	var paths []string
	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		if excludePath != "" && p.Path == excludePath {
			continue
		}
		paths = append(paths, p.Path)
	}
	return paths
}

// ExtractToken finds the temenos_token annotation in a task's annotations.
// Returns empty string if not found.
func ExtractToken(annotations []taskwarrior.Annotation) string {
	const prefix = "temenos_token:"
	for _, ann := range annotations {
		if strings.HasPrefix(ann.Description, prefix) {
			return strings.TrimPrefix(ann.Description, prefix)
		}
	}
	return ""
}

// DeleteSessionByToken is a convenience wrapper that creates a new client and
// deletes the session for the given token.
func DeleteSessionByToken(ctx context.Context, token string) error {
	return New("").DeleteSession(ctx, token)
}

// mcpConfigDir returns the directory where MCP config files are stored.
// Naming convention: m.json for managers (shared — all have identical permissions),
// w-<hexid>.json for workers (per-task, deleted on close).
func mcpConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ttal", "mcps"), nil
}

// ManagerMCPConfigPath returns the path to the shared manager MCP config file.
// All manager agents share this file — token lifecycle is tied to the daemon, not individual agents.
// Returns empty string (logged as warning) if the home directory cannot be determined.
func ManagerMCPConfigPath() string {
	dir, err := mcpConfigDir()
	if err != nil {
		log.Printf("[temenos] warning: cannot resolve MCP config dir: %v", err)
		return ""
	}
	return filepath.Join(dir, "m.json")
}

// ReadMCPConfigToken reads the session token embedded in ~/.ttal/mcps/<name>.json.
// Returns empty string if the file does not exist or cannot be parsed.
func ReadMCPConfigToken(name string) string {
	dir, err := mcpConfigDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return ""
	}
	var doc mcpConfigDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	if s, ok := doc.MCPServers["temenos"]; ok {
		return s.Headers["X-Session-Token"]
	}
	return ""
}

// WriteMCPConfigFile writes mcpJSON to ~/.ttal/mcps/<name>.json and returns the path.
// Use "m" for the shared manager config and "w-<hexid>" for per-worker configs.
func WriteMCPConfigFile(name, mcpJSON string) (string, error) {
	dir, err := mcpConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create mcp config dir: %w", err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, []byte(mcpJSON), 0o600); err != nil {
		return "", fmt.Errorf("write mcp config file %s: %w", path, err)
	}
	return path, nil
}

// DeleteMCPConfigFile removes ~/.ttal/mcps/<name>.json. Best-effort: no error returned.
// Logs a warning for unexpected errors (not-exist is silently ignored).
func DeleteMCPConfigFile(name string) {
	dir, err := mcpConfigDir()
	if err != nil {
		log.Printf("[temenos] warning: cannot resolve MCP config dir for delete: %v", err)
		return
	}
	path := filepath.Join(dir, name+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("[temenos] warning: failed to delete MCP config %s: %v", path, err)
	}
}
