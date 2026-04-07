package temenos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	envpkg "github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const defaultMCPPort = 9783

// Token annotation prefixes used to store session tokens on tasks for cleanup.
const (
	TokenAnnotationWorker       = "temenos_token:"
	TokenAnnotationPRReviewer   = "temenos_pr_reviewer_token:"
	TokenAnnotationPlanReviewer = "temenos_plan_reviewer_token:"
)

// Env key constants for session environment variables.
const (
	EnvKeyAgentName = "TTAL_AGENT_NAME"
	EnvKeyJobID     = "TTAL_JOB_ID"
	EnvKeyRuntime   = "TTAL_RUNTIME"
	EnvKeyTaskRC    = "TASKRC"
)

// BuildSessionEnv returns the session-scoped env map for a CC worker, manager, or reviewer.
// Includes identity vars and allowlisted .env vars. API tokens are excluded by the filter.
func BuildSessionEnv(agentName, jobID string, rt runtime.Runtime, taskRC string) map[string]string {
	m := map[string]string{
		EnvKeyAgentName: agentName,
		EnvKeyJobID:     jobID,
		EnvKeyRuntime:   string(rt),
	}
	if taskRC != "" {
		m[EnvKeyTaskRC] = taskRC
	}
	if dotEnv := envpkg.AllowedDotEnvMap(); dotEnv != nil {
		for k, v := range dotEnv {
			m[k] = v
		}
	}
	return m
}

// RegisterReviewerTemenos registers a temenos session for a reviewer, annotates the task,
// writes the MCP config, and returns the path. Rolls back the session on annotation or
// config-write failure (best-effort).
func RegisterReviewerTemenos(
	ctx context.Context, reviewerName string, workDir string,
	jobID string, annotationPrefix string, rt runtime.Runtime,
) string {
	env := BuildSessionEnv(reviewerName, jobID, rt, "")
	mcpJSON, token, err := RegisterSessionForAgent(ctx, reviewerName, []string{workDir}, "", env)
	if err != nil {
		log.Printf("[temenos] warning: failed to register reviewer session (non-fatal): %v", err)
		return ""
	}

	// Annotate task with token for cleanup on close.
	// On failure, roll back the session to avoid leaking it.
	if annErr := taskwarrior.AnnotateTask(jobID, annotationPrefix+token); annErr != nil {
		log.Printf("[temenos] warning: reviewer token annotation failed — rolling back session: %v", annErr)
		if rollbackErr := DeleteSessionByTokenWithTimeout(ctx, token); rollbackErr != nil {
			log.Printf("[temenos] warning: session rollback failed (TTL will reclaim): %v", rollbackErr)
		}
		return ""
	}

	mcpName := ReviewerMCPName(jobID, reviewerRoleFromPrefix(annotationPrefix))
	path, err := WriteMCPConfigFile(mcpName, mcpJSON)
	if err != nil {
		log.Printf("[temenos] warning: reviewer MCP config write failed (reviewer runs without MCP): %v", err)
		// Session and annotation persist — reviewer spawns without MCP. Cleanup will find the token.
		return ""
	}
	return path
}

// reviewerRoleFromPrefix maps a token annotation prefix to the reviewer role string.
func reviewerRoleFromPrefix(prefix string) string {
	switch prefix {
	case TokenAnnotationPRReviewer:
		return "pr"
	case TokenAnnotationPlanReviewer:
		return "plan"
	default:
		return "reviewer"
	}
}

// DeleteSessionByTokenWithTimeout deletes a session using a fresh 5s-timeout context.
func DeleteSessionByTokenWithTimeout(ctx context.Context, token string) error {
	deleteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return DeleteSessionByToken(deleteCtx, token)
}

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
// env is an optional map of session-scoped environment variables.
//
// Returns the MCP config JSON, the session token, and any error.
func RegisterSessionForAgent(
	ctx context.Context, agent string, writePaths []string, excludeReadPath string, env map[string]string,
) (mcpJSON, token string, err error) {
	readPaths := gatherReadPaths(excludeReadPath)

	c := New("")
	token, err = c.RegisterSession(ctx, agent, writePaths, readPaths, env)
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

// AgentMCPConfigPath returns the per-agent MCP config file path for a manager agent.
// Use this instead of ManagerMCPConfigPath when each agent needs its own session token.
func AgentMCPConfigPath(agentName string) string {
	dir, err := mcpConfigDir()
	if err != nil {
		log.Printf("[temenos] warning: cannot resolve MCP config dir: %v", err)
		return ""
	}
	return filepath.Join(dir, agentName+".json")
}

// ReviewerMCPName returns the MCP config file name for a reviewer session.
// role is "pr" for PR reviewers or "plan" for plan reviewers.
func ReviewerMCPName(taskHexID, role string) string {
	return "r-" + taskHexID + "-" + role
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
