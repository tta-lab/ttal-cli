package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tta-lab/ttal-cli/internal/claudeconfig"
	"github.com/tta-lab/ttal-cli/internal/config"
	envpkg "github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	codexRuntime "github.com/tta-lab/ttal-cli/internal/runtime/codex"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/temenos"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// initAdapters starts all agent sessions in parallel.
// CC agents use tmux sessions; Codex agents use the WebSocket adapter.
// Returns a map of agentName → mcpFilePath for CC agents, used during shutdown.
func initAdapters(
	ctx context.Context, mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend, msgSvc *message.Service,
) map[string]string {
	ensureLocalAgentTrust(mcfg)

	// Register per-agent MCP tokens at daemon start.
	// Each CC manager agent gets its own temenos session with its own env.
	mcpPaths := initManagerMCPTokens(ctx, mcfg)

	var wg sync.WaitGroup
	for _, ta := range mcfg.AllAgents() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initSingleAdapter(ctx, ta, mcfg, registry, frontends, msgSvc, mcpPaths)
		}()
	}
	wg.Wait()
	return mcpPaths
}

// initSingleAdapter initializes a single agent's session (CC via tmux, or Codex via adapter).
func initSingleAdapter(
	ctx context.Context, ta config.TeamAgent, mcfg *config.DaemonConfig,
	registry *adapterRegistry, frontends map[string]frontend.Frontend,
	msgSvc *message.Service, mcpPaths map[string]string,
) {
	agentPath := filepath.Join(ta.TeamPath, ta.AgentName)

	rt := mcfg.RuntimeForAgent(ta.TeamName, ta.TeamPath, ta.AgentName)

	// CC agents use tmux
	if rt == runtime.ClaudeCode {
		sessionName := config.AgentSessionName(ta.TeamName, ta.AgentName)
		if tmux.SessionExists(sessionName) {
			log.Printf("[daemon] CC agent %s already running (session: %s)", ta.AgentName, sessionName)
			return
		}
		agentEnv := buildManagerAgentEnv(ta.AgentName, ta.TeamName, mcfg)
		shell := mcfg.Global.GetShell()
		ensureProjectDir(agentPath)
		mcpPath := mcpPaths[ta.AgentName]
		// Resume from last session if available.
		resumeSessionID := lastSessionID(ta.TeamName, ta.AgentName, agentPath)
		if err := spawnCCSession(
			sessionName, ta.AgentName, agentPath,
			agentEnv, shell, mcpPath, resumeSessionID,
		); err != nil {
			log.Printf("[daemon] failed to start CC session for %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] CC agent %s running (session: %s)", ta.AgentName, sessionName)
		}
		return
	}

	// Codex agents use the WebSocket adapter
	if rt == runtime.Codex {
		cfg := runtime.AdapterConfig{
			AgentName: ta.AgentName,
			WorkDir:   agentPath,
			Env:       buildManagerAgentEnv(ta.AgentName, ta.TeamName, mcfg),
			TeamPath:  ta.TeamPath,
		}
		adapter := codexRuntime.New(cfg)
		if err := adapter.Start(ctx); err != nil {
			log.Printf("[daemon] failed to start Codex adapter for %s: %v", ta.AgentName, err)
			return
		}
		registry.set(ta.TeamName, ta.AgentName, adapter)
		initCodexSession(ctx, ta.AgentName, adapter)
		go bridgeAdapterEvents(ctx, ta.TeamName, ta.AgentName, adapter, mcfg, frontends, msgSvc)
		log.Printf("[daemon] Codex agent %s running", ta.AgentName)
		return
	}
}

// initCodexSession tries to resume the last thread or creates a new session.
func initCodexSession(ctx context.Context, agentName string, adapter *codexRuntime.Adapter) {
	if lastID, err := adapter.ListThreads(ctx); err == nil && lastID != "" {
		if _, err := adapter.ResumeSession(ctx, lastID); err == nil {
			log.Printf("[daemon] Codex agent %s resumed thread %s", agentName, lastID)
			return
		}
		log.Printf("[daemon] Codex agent %s failed to resume thread %s: %v", agentName, lastID, err)
	}
	if _, err := adapter.CreateSession(ctx); err != nil {
		log.Printf("[daemon] Codex agent %s failed to create session: %v", agentName, err)
	}
}

// bridgeAdapterEvents routes Codex adapter events to frontends and status.
func bridgeAdapterEvents(
	ctx context.Context, teamName, agentName string, adapter *codexRuntime.Adapter,
	mcfg *config.DaemonConfig, frontends map[string]frontend.Frontend, msgSvc *message.Service,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-adapter.Events():
			if !ok {
				return
			}
			fe, hasFE := frontends[teamName]
			switch evt.Type {
			case runtime.EventText:
				if hasFE {
					persistMsg(msgSvc, message.CreateParams{
						Sender: agentName, Recipient: mcfg.Global.UserName(),
						Content: evt.Text, Team: teamName, Channel: message.ChannelCLI,
					})
					_ = fe.SendText(ctx, agentName, evt.Text)
				}
			case runtime.EventError:
				log.Printf("[daemon] codex error for %s: %s", agentName, evt.Text)
			case runtime.EventTool:
				if hasFE {
					teamCfg := mcfg.Teams[teamName]
					if teamCfg != nil && teamCfg.EmojiReactions {
						emoji := telegram.ToolEmoji(evt.ToolName)
						if emoji != "" {
							_ = fe.SetReaction(ctx, agentName, emoji)
						}
					}
				}
			case runtime.EventStatus:
				if err := status.WriteAgent(teamName, status.AgentStatus{
					Agent:               agentName,
					ContextUsedPct:      evt.ContextUsedPct,
					ContextRemainingPct: evt.ContextRemainingPct,
					UpdatedAt:           time.Now(),
				}); err != nil {
					log.Printf("[daemon] codex status write error for %s: %v", agentName, err)
				}
			}
		}
	}
}

// gatherProjectPaths returns all active project paths across all teams.
// storePathFn maps a team name to the projects.toml path for that team.
func gatherProjectPaths(mcfg *config.DaemonConfig, storePathFn func(string) string) []string {
	seen := make(map[string]bool)
	var paths []string

	for teamName := range mcfg.Teams {
		projectsPath := storePathFn(teamName)
		store := project.NewStore(projectsPath)
		projects, err := store.List(false)
		if err != nil {
			log.Printf("[daemon] warning: failed to load projects for team %s: %v", teamName, err)
			continue
		}
		for _, p := range projects {
			if p.Path != "" && !seen[p.Path] {
				seen[p.Path] = true
				paths = append(paths, p.Path)
			}
		}
	}

	sort.Strings(paths)
	return paths
}

// buildManagerAgentEnv returns env vars for a manager agent session.
func buildManagerAgentEnv(agentName, teamName string, mcfg *config.DaemonConfig) []string {
	agentEnv := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
	}
	if team, ok := mcfg.Teams[teamName]; ok && team.TaskRC != "" {
		agentEnv = append(agentEnv, fmt.Sprintf("TASKRC=%s", team.TaskRC))
	}
	// Inject allowlisted .env vars — tokens stay in daemon, not agent sessions.
	agentEnv = append(agentEnv, envpkg.AllowedDotEnvParts()...)
	return agentEnv
}

// ensureLocalAgentTrust adds hasTrustDialogAccepted entries to ~/.claude.json
// for all agent workspace paths. Idempotent.
func ensureLocalAgentTrust(mcfg *config.DaemonConfig) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[daemon] warning: cannot get home dir for local agent trust: %v", err)
		return
	}

	var paths []string
	for _, ta := range mcfg.AllAgents() {
		paths = append(paths, filepath.Join(ta.TeamPath, ta.AgentName))
	}
	if len(paths) == 0 {
		return
	}

	claudeJSONPath := filepath.Join(home, ".claude.json")
	added, err := claudeconfig.UpsertTrust(claudeJSONPath, paths)
	if err != nil {
		log.Printf("[daemon] warning: could not update agent trust in %s: %v\n"+
			"  — delete the file to reset or check permissions", claudeJSONPath, err)
		return
	}
	if added > 0 {
		log.Printf("[daemon] added trust entries for %d local agent workspaces", added)
	}
}

// shutdownAgents gracefully shuts down all agent sessions on daemon exit.
// Local CC sessions are killed directly; status files are preserved so the
// next spawn can resume with --resume <session-id>.
// Also unregisters per-agent temenos MCP tokens.
func shutdownAgents(mcfg *config.DaemonConfig, registry *adapterRegistry, mcpPaths map[string]string) {
	registry.stopAll(context.Background())
	sessions := collectCCSessions(mcfg)
	if len(sessions) > 0 {
		shutdownCCSessions(sessions)
	}
	shutdownManagerMCPTokens(context.Background(), mcpPaths)
}

// collectCCSessions returns running CC tmux session names across all teams.
func collectCCSessions(mcfg *config.DaemonConfig) []string {
	var sessions []string
	for _, ta := range mcfg.AllAgents() {
		rt := mcfg.RuntimeForAgent(ta.TeamName, ta.TeamPath, ta.AgentName)
		if rt != runtime.ClaudeCode {
			continue
		}
		sessionName := config.AgentSessionName(ta.TeamName, ta.AgentName)
		if !tmux.SessionExists(sessionName) {
			continue
		}
		sessions = append(sessions, sessionName)
	}
	return sessions
}

// shutdownCCSessions kills CC tmux sessions directly.
func shutdownCCSessions(sessions []string) {
	for _, s := range sessions {
		if err := tmux.KillSession(s); err != nil {
			log.Printf("[daemon] failed to kill session %s: %v", s, err)
		} else {
			log.Printf("[daemon] killed CC session %s", s)
		}
	}
}

// agentProjectDir returns the ~/.claude/projects/<encoded> path for an agent.
func agentProjectDir(agentPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	encoded := watcher.EncodePath(agentPath)
	return filepath.Join(home, ".claude", "projects", encoded), nil
}

// ensureProjectDir creates the CC JSONL project directory for an agent.
// Called before spawnCCSession so the dir exists when CC starts and is
// ready for the watcher to monitor.
func ensureProjectDir(agentPath string) {
	dir, err := agentProjectDir(agentPath)
	if err != nil {
		log.Printf("[daemon] failed to resolve project dir for %s: %v", filepath.Base(agentPath), err)
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("[daemon] failed to create project dir for %s: %v", filepath.Base(agentPath), err)
	}
}

// initManagerMCPTokens registers a per-agent temenos session for each CC manager agent.
// Returns a map of agentName → mcpFilePath for passing to spawnCCSession.
// Best-effort: agents still launch without MCP on error.
func initManagerMCPTokens(ctx context.Context, mcfg *config.DaemonConfig) map[string]string {
	mcpPaths := make(map[string]string)

	for _, ta := range mcfg.AllAgents() {
		rt := mcfg.RuntimeForAgent(ta.TeamName, ta.TeamPath, ta.AgentName)
		if rt != runtime.ClaudeCode {
			continue
		}

		// Unregister any previous token for this agent.
		if token := temenos.ReadMCPConfigToken(ta.AgentName); token != "" {
			if err := temenos.DeleteSessionByToken(ctx, token); err != nil {
				log.Printf("[daemon] warning: failed to unregister old token for %s (non-fatal): %v", ta.AgentName, err)
			}
		}

		// Build env for this agent.
		envSlice := buildManagerAgentEnv(ta.AgentName, ta.TeamName, mcfg)
		envMap := envpkg.EnvSliceToMap(envSlice)

		mcpJSON, _, err := temenos.RegisterSessionForAgent(ctx, ta.AgentName, nil, "", envMap)
		if err != nil {
			log.Printf("[daemon] warning: failed to register temenos session for %s (non-fatal): %v", ta.AgentName, err)
			continue
		}

		path, err := temenos.WriteMCPConfigFile(ta.AgentName, mcpJSON)
		if err != nil {
			log.Printf("[daemon] warning: failed to write MCP config for %s (non-fatal): %v", ta.AgentName, err)
			// Clean up the stale registration to avoid leaking tokens.
			temenos.DeleteMCPConfigFile(ta.AgentName)
			continue
		}
		mcpPaths[ta.AgentName] = path
	}

	return mcpPaths
}

// shutdownManagerMCPTokens unregisters and deletes per-agent MCP config files.
func shutdownManagerMCPTokens(ctx context.Context, mcpPaths map[string]string) {
	if mcpPaths == nil {
		return
	}
	for agentName, mcpPath := range mcpPaths {
		// Unregister the temenos session.
		if token := temenos.ReadMCPConfigToken(agentName); token != "" {
			if err := temenos.DeleteSessionByToken(ctx, token); err != nil {
				log.Printf("[daemon] warning: failed to unregister MCP token for %s: %v", agentName, err)
			}
		}
		// Delete the config file.
		temenos.DeleteMCPConfigFile(agentName)
		_ = mcpPath // suppress unused warning; path known via agentName
	}
}

// spawnCCSession creates a tmux session for a Claude Code agent.
// mcpConfig, if non-empty, is appended to the claude command via --mcp-config.
// resumeSessionID, if non-empty, appends --resume <id> (cold-start resume path).
// When resumeSessionID is empty, starts a fresh session (breathe restart path).
func spawnCCSession(
	sessionName, agentName, agentPath string,
	env []string, shell, mcpConfig, resumeSessionID string,
) error {
	cmd := "claude --dangerously-skip-permissions --agent " + agentName
	if resumeSessionID != "" {
		cmd += " --resume " + resumeSessionID
	}
	cmd = launchcmd.AppendMCPConfig(cmd, mcpConfig)

	envStr := ""
	if len(env) > 0 {
		envStr = fmt.Sprintf("env %s ", strings.Join(env, " "))
	}
	var shellCmd string
	switch shell {
	case "fish":
		shellCmd = fmt.Sprintf("%sfish -C '%s'", envStr, cmd)
	default:
		shellCmd = fmt.Sprintf("%szsh -c '%s'", envStr, cmd)
	}

	if err := tmux.NewSession(sessionName, agentName, agentPath, shellCmd); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			_ = tmux.SetEnv(sessionName, parts[0], parts[1])
		}
	}
	return nil
}

// sessionJSONLExists checks if a session's JSONL file exists in the
// project dir for the given agent path.
// Returns true on unexpected stat errors (conservative fallback — better to
// attempt --resume than silently drop it on a transient I/O error).
func sessionJSONLExists(sessionID, agentPath string) bool {
	dir, err := agentProjectDir(agentPath)
	if err != nil {
		return true // best-effort: assume exists
	}
	jsonlPath := filepath.Join(dir, sessionID+".jsonl")
	_, err = os.Stat(jsonlPath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Printf("[daemon] WARN: could not stat session JSONL %s: %v — assuming exists", jsonlPath, err)
	return true
}

// lastSessionID reads the persisted CC session ID for an agent from the status file.
// Returns "" on cold-start (no prior session), on read error (logged as WARN),
// or when the session's JSONL doesn't exist in the current project dir (CWD change).
func lastSessionID(teamName, agentName, agentPath string) string {
	s, err := status.ReadAgent(teamName, agentName)
	if err != nil {
		log.Printf("[daemon] WARN: could not read status for %s/%s, skipping --resume: %v", teamName, agentName, err)
		return ""
	}
	if s == nil {
		// Cold start — no prior session, nothing to resume.
		return ""
	}
	// Verify session JSONL exists in the current project dir.
	// After a CWD change the old session lives in a different encoded dir.
	if !sessionJSONLExists(s.SessionID, agentPath) {
		dir, _ := agentProjectDir(agentPath)
		log.Printf("[daemon] session %s not found in %s — starting fresh", s.SessionID, filepath.Base(dir))
		return ""
	}
	return s.SessionID
}
