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
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	codexRuntime "github.com/tta-lab/ttal-cli/internal/runtime/codex"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// initAdapters starts all agent sessions in parallel.
// CC agents use tmux sessions; Codex agents use the WebSocket adapter.
func initAdapters(
	ctx context.Context, cfg *config.Config, registry *adapterRegistry,
	frontends map[string]frontend.Frontend, msgSvc *message.Service,
) {
	ensureLocalAgentTrust(cfg)

	var wg sync.WaitGroup
	for _, ta := range cfg.Agents() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initSingleAdapter(ctx, ta, cfg, registry, frontends, msgSvc)
		}()
	}
	wg.Wait()
}

// initSingleAdapter initializes a single agent's session (CC via tmux, or Codex via adapter).
func initSingleAdapter(
	ctx context.Context, ta config.AgentInfo, cfg *config.Config,
	registry *adapterRegistry, frontends map[string]frontend.Frontend,
	msgSvc *message.Service,
) {
	agentPath := filepath.Join(ta.TeamPath, ta.AgentName)

	rt := cfg.RuntimeForAgent(ta.AgentName)

	// CC agents use tmux
	if rt == runtime.ClaudeCode {
		sessionName := config.AgentSessionName(ta.AgentName)
		if tmux.SessionExists(sessionName) {
			log.Printf("[daemon] CC agent %s already running (session: %s)", ta.AgentName, sessionName)
			return
		}
		agentEnv := buildManagerAgentEnv(ta.AgentName, cfg)
		shell := cfg.GetShell()
		ensureProjectDir(agentPath)
		// Resume from last session if available.
		resumeSessionID := lastSessionID(ta.AgentName, agentPath)
		if err := spawnCCSession(
			sessionName, ta.AgentName, agentPath,
			agentEnv, shell, resumeSessionID,
		); err != nil {
			log.Printf("[daemon] failed to start CC session for %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] CC agent %s running (session: %s)", ta.AgentName, sessionName)
		}
		return
	}

	// Codex agents use the WebSocket adapter
	if rt == runtime.Codex {
		codexCfg := runtime.AdapterConfig{
			AgentName: ta.AgentName,
			WorkDir:   agentPath,
			Env:       buildManagerAgentEnv(ta.AgentName, cfg),
			TeamPath:  ta.TeamPath,
		}
		adapter := codexRuntime.New(codexCfg)
		if err := adapter.Start(ctx); err != nil {
			log.Printf("[daemon] failed to start Codex adapter for %s: %v", ta.AgentName, err)
			return
		}
		registry.set("default", ta.AgentName, adapter)
		initCodexSession(ctx, ta.AgentName, adapter)
		go bridgeAdapterEvents(ctx, ta.AgentName, adapter, cfg, frontends, msgSvc)
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
	ctx context.Context, agentName string, adapter *codexRuntime.Adapter,
	cfg *config.Config, frontends map[string]frontend.Frontend, msgSvc *message.Service,
) {
	fe, hasFE := frontends["default"]
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-adapter.Events():
			if !ok {
				return
			}
			switch evt.Type {
			case runtime.EventText:
				if hasFE {
					persistMsg(msgSvc, message.CreateParams{
						Sender: agentName, Recipient: cfg.UserName,
						Content: evt.Text, Team: "default", Channel: message.ChannelCLI,
					})
					_ = fe.SendText(ctx, agentName, evt.Text)
				}
			case runtime.EventError:
				log.Printf("[daemon] codex error for %s: %s", agentName, evt.Text)
			case runtime.EventTool:
				if hasFE && cfg.EmojiReactions {
					emoji := telegram.ToolEmoji(evt.ToolName)
					if emoji != "" {
						_ = fe.SetReaction(ctx, agentName, emoji)
					}
				}
			case runtime.EventStatus:
				if err := status.WriteAgent("default", status.AgentStatus{
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

// gatherProjectPaths returns all active project paths.
// Single-team: only "default" team.
func gatherProjectPaths(_ *config.Config, storePathFn func(string) string) []string {
	seen := make(map[string]bool)
	var paths []string

	projectsPath := storePathFn("default")
	store := project.NewStore(projectsPath)
	projects, err := store.List(false)
	if err != nil {
		log.Printf("[daemon] warning: failed to load projects for team %s: %v", "default", err)
	} else {
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
func buildManagerAgentEnv(agentName string, cfg *config.Config) []string {
	parts := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		// Opt into Claude Code's longer prompt cache TTL for manager sessions.
		// Managers routinely pause >5 min between tool calls; the longer TTL
		// survives those gaps and avoids full-prefix rewrites that inflate cost
		// and quota. Requires CC >= 2.1.108 (no-op on older versions).
		"ENABLE_PROMPT_CACHING_1H=1",
	}
	if cfg != nil && cfg.AdminHuman != nil {
		parts = append(parts, "TTAL_HUMAN="+cfg.AdminHuman.Alias)
		parts = append(parts, "TTAL_ADMIN_NAME="+cfg.AdminHuman.Name)
		parts = append(parts, "TTAL_ADMIN_HANDLE="+cfg.AdminHuman.Alias)
	}
	return parts
}

// ensureLocalAgentTrust adds hasTrustDialogAccepted entries to ~/.claude.json
// for all agent workspace paths. Idempotent.
func ensureLocalAgentTrust(cfg *config.Config) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[daemon] warning: cannot get home dir for local agent trust: %v", err)
		return
	}

	var paths []string
	for _, ta := range cfg.Agents() {
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
func shutdownAgents(cfg *config.Config, registry *adapterRegistry) {
	registry.stopAll(context.Background())
	sessions := collectCCSessions(cfg)
	if len(sessions) > 0 {
		shutdownCCSessions(sessions)
	}
}

// collectCCSessions returns running CC tmux session names across all teams.
func collectCCSessions(cfg *config.Config) []string {
	var sessions []string
	for _, ta := range cfg.Agents() {
		rt := cfg.RuntimeForAgent(ta.AgentName)
		if rt != runtime.ClaudeCode {
			continue
		}
		sessionName := config.AgentSessionName(ta.AgentName)
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
	encoded := encodeAgentPath(agentPath)
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

// spawnCCSession creates a tmux session for a Claude Code agent.
// resumeSessionID, if non-empty, appends --resume <id> (cold-start resume path).
// When resumeSessionID is empty, starts a fresh session (breathe restart path).
func spawnCCSession(
	sessionName, agentName, agentPath string,
	env []string, shell, resumeSessionID string,
) error {
	cmd := "claude --dangerously-skip-permissions --agent " + agentName
	if resumeSessionID != "" {
		cmd += " --resume " + resumeSessionID
	}

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
func lastSessionID(agentName, agentPath string) string {
	s, err := status.ReadAgent("default", agentName)
	if err != nil {
		log.Printf("[daemon] WARN: could not read status for %s, skipping --resume: %v", agentName, err)
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
