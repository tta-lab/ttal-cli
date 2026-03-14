package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/claudeconfig"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// initAdapters starts all agent sessions in parallel via tmux.
// Config-driven: iterates all teams, no DB required.
func initAdapters(mcfg *config.DaemonConfig) {
	ensureLocalAgentTrust(mcfg)

	// Spawn per-agent sessions in parallel
	var wg sync.WaitGroup
	for _, ta := range mcfg.AllAgents() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initSingleAdapter(ta, mcfg)
		}()
	}
	wg.Wait()
}

// initSingleAdapter initializes a single agent's tmux session.
func initSingleAdapter(
	ta config.TeamAgent, mcfg *config.DaemonConfig,
) {
	agentPath := filepath.Join(ta.TeamPath, ta.AgentName)

	rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)

	// CC agents use tmux — spawn session but don't register adapter
	// (deliverToAgent falls back to tmux send-keys for unregistered agents).
	if rt == runtime.ClaudeCode {
		sessionName := config.AgentSessionName(ta.TeamName, ta.AgentName)
		if tmux.SessionExists(sessionName) {
			log.Printf("[daemon] CC agent %s already running (session: %s)", ta.AgentName, sessionName)
			return
		}
		model := mcfg.AgentModelForTeam(ta.TeamName, ta.AgentName)
		env := buildAgentEnv(ta.AgentName, ta.TeamName, mcfg)
		shell := mcfg.Global.GetShell()
		ensureProjectDir(agentPath)
		if err := spawnCCSession(sessionName, ta.AgentName, agentPath, model, ta.TeamName, env, shell); err != nil {
			log.Printf("[daemon] failed to start CC session for %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] CC agent %s running (session: %s)", ta.AgentName, sessionName)
		}
		return
	}
}

// buildAgentEnv returns env vars for an agent adapter.
func buildAgentEnv(agentName, teamName string, mcfg *config.DaemonConfig) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		fmt.Sprintf("TTAL_TEAM=%s", teamName),
	}
	if team, ok := mcfg.Teams[teamName]; ok && team.TaskRC != "" {
		env = append(env, fmt.Sprintf("TASKRC=%s", team.TaskRC))
	}
	// Read flicknote_project from CLAUDE.md frontmatter
	if team, ok := mcfg.Teams[teamName]; ok && team.TeamPath != "" {
		info, err := agentfs.GetFromPath(filepath.Join(team.TeamPath, agentName))
		if err == nil && info.FlicknoteProject != "" {
			env = append(env, fmt.Sprintf("FLICKNOTE_PROJECT=%s", info.FlicknoteProject))
		}
	}

	// Inject all secrets from .env
	env = append(env, config.DotEnvParts()...)

	return env
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
func shutdownAgents(mcfg *config.DaemonConfig, registry *adapterRegistry) {
	registry.stopAll(context.Background())
	sessions := collectCCSessions(mcfg)
	if len(sessions) > 0 {
		shutdownCCSessions(sessions)
	}
}

// collectCCSessions returns running CC tmux session names across all teams.
func collectCCSessions(mcfg *config.DaemonConfig) []string {
	var sessions []string
	for _, ta := range mcfg.AllAgents() {
		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)
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

// spawnCCSession creates a tmux session for a Claude Code agent.
func spawnCCSession(sessionName, agentName, agentPath, model, teamName string, env []string, shell string) error {
	cmd := "claude --dangerously-skip-permissions --agent " + agentName
	if model != "" {
		cmd += " --model " + model
	}
	if sid := lastSessionID(teamName, agentName, agentPath); sid != "" {
		cmd += " --resume " + sid
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
