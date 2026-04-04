package temenos

import (
	"context"
	"encoding/json"
	"fmt"
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
