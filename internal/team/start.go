package team

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/status"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"

	entagent "codeberg.org/clawteam/ttal-cli/ent/agent"
)

// AgentTab holds the info needed to create a tab for one agent.
type AgentTab struct {
	Name    string
	Path    string
	Model   string
	Runtime runtime.Runtime
	Port    int
}

// Start creates per-agent tmux sessions (one session per agent).
// Without --force: skips already-running sessions, only starts missing ones.
// With --force: kills and recreates all sessions.
func Start(database *ent.Client, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	started := make([]AgentTab, 0, len(cfg.Agents))
	skipped := make([]string, 0, len(cfg.Agents))

	for agentName := range cfg.Agents {
		agentPath := cfg.AgentPath(agentName)
		if agentPath == "" {
			fmt.Fprintf(os.Stderr, "warning: agent %q has no path (set team_path in config), skipping\n", agentName)
			continue
		}

		ag, err := database.Agent.Query().
			Where(entagent.Name(agentName)).
			Only(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: agent %q not found in ttal DB, skipping\n", agentName)
			continue
		}

		// Precedence: agent DB runtime > team agent_runtime > "claude-code"
		rt := cfg.AgentRuntime()
		if ag.Runtime != nil {
			rt = runtime.Runtime(*ag.Runtime)
		}
		port := cfg.Agents[agentName].Port
		tab := AgentTab{Name: agentName, Path: agentPath, Model: string(ag.Model), Runtime: rt, Port: port}
		sessionName := config.AgentSessionName(cfg.TeamName(), agentName)

		if tmux.SessionExists(sessionName) {
			if !force {
				skipped = append(skipped, agentName)
				continue
			}
			fmt.Printf("Removing existing session %q...\n", sessionName)
			if err := tmux.KillSession(sessionName); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove session %q: %v\n", sessionName, err)
				continue
			}
			status.Remove(agentName)
		}

		if err := launchAgentSession(sessionName, tab, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", agentName, err)
			continue
		}

		started = append(started, tab)
	}

	if len(started) == 0 && len(skipped) == 0 {
		return fmt.Errorf("no agent sessions started — set team_path in config and register agents with: ttal agent add <name>")
	}

	if len(started) > 0 {
		fmt.Printf("Started %d agent sessions:\n", len(started))
		for _, t := range started {
			fmt.Printf("  %s → %s (session: %s)\n", t.Name, t.Path, config.AgentSessionName(cfg.TeamName(), t.Name))
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d already running: %s\n", len(skipped), strings.Join(skipped, ", "))
	}
	fmt.Printf("\nAttach with: ttal team attach <agent-name>\n")

	return nil
}

// launchAgentSession creates a tmux session for CC agents, or verifies
// daemon management for OC/Codex agents.
func launchAgentSession(sessionName string, tab AgentTab, cfg *config.Config) error {
	switch tab.Runtime {
	case runtime.OpenClaw:
		// OpenClaw manages its own sessions — nothing to spawn.
		fmt.Printf("  %s agent %s managed by OpenClaw\n", tab.Runtime, tab.Name)
		return nil
	case runtime.OpenCode, runtime.Codex:
		// OC/Codex agents are managed by the daemon, not tmux.
		if running, _, _ := daemon.IsRunning(); !running {
			return fmt.Errorf("daemon not running — start with: ttal daemon run")
		}
		fmt.Printf("  %s agent %s managed by daemon (port %d)\n", tab.Runtime, tab.Name, tab.Port)
		return nil
	default:
		return launchCCAgentSession(sessionName, tab, cfg)
	}
}

// launchCCAgentSession creates a tmux session for a Claude Code agent.
func launchCCAgentSession(sessionName string, tab AgentTab, cfg *config.Config) error {
	agentCmd := buildClaudeCodeAgentCommand(tab)

	envParts := []string{fmt.Sprintf("TTAL_AGENT_NAME=%s", tab.Name)}
	if team := cfg.TeamName(); team != "default" || os.Getenv("TTAL_TEAM") != "" {
		envParts = append(envParts, fmt.Sprintf("TTAL_TEAM=%s", team))
	}
	if taskrc := cfg.TaskRC(); taskrc != config.DefaultTaskRC() {
		envParts = append(envParts, fmt.Sprintf("TASKRC=%s", taskrc))
	}

	shellCmd := cfg.BuildEnvShellCommand(envParts, agentCmd)
	if err := tmux.NewSession(sessionName, tab.Name, tab.Path, shellCmd); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if team := cfg.TeamName(); team != "default" || os.Getenv("TTAL_TEAM") != "" {
		_ = tmux.SetEnv(sessionName, "TTAL_TEAM", team)
	}
	if taskrc := cfg.TaskRC(); taskrc != config.DefaultTaskRC() {
		_ = tmux.SetEnv(sessionName, "TASKRC", taskrc)
	}

	return nil
}

func buildClaudeCodeAgentCommand(tab AgentTab) string {
	cmd := "claude --dangerously-skip-permissions"
	if tab.Model != "" {
		cmd += " --model " + tab.Model
	}
	if hasClaudeConversation(tab.Path) {
		cmd += " --continue"
	}
	return cmd
}

// hasClaudeConversation checks if Claude Code has a previous conversation for the given path.
// Claude stores conversations as .jsonl files in ~/.claude/projects/<sanitized-path>/.
func hasClaudeConversation(workDir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Claude sanitizes paths: / and . → -
	sanitized := strings.ReplaceAll(workDir, string(filepath.Separator), "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	projectDir := filepath.Join(home, ".claude", "projects", sanitized)

	matches, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return false
	}
	return len(matches) > 0
}
