package team

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/status"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"

	entagent "codeberg.org/clawteam/ttal-cli/ent/agent"
)

// AgentTab holds the info needed to create a tab for one agent.
type AgentTab struct {
	Name  string
	Path  string
	Model string
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
		ag, err := database.Agent.Query().
			Where(entagent.Name(agentName)).
			Only(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: agent %q not found in ttal DB, skipping\n", agentName)
			continue
		}
		if ag.Path == "" {
			fmt.Fprintf(os.Stderr, "warning: agent %q has no path, skipping\n", agentName)
			continue
		}

		tab := AgentTab{Name: agentName, Path: ag.Path, Model: string(ag.Model)}
		sessionName := config.AgentSessionName(agentName)

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
		return fmt.Errorf("no agent sessions started — register agents with: ttal agent add <name> --path <path>")
	}

	if len(started) > 0 {
		fmt.Printf("Started %d agent sessions:\n", len(started))
		for _, t := range started {
			fmt.Printf("  %s → %s (session: %s)\n", t.Name, t.Path, config.AgentSessionName(t.Name))
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d already running: %s\n", len(skipped), strings.Join(skipped, ", "))
	}
	fmt.Printf("\nAttach with: ttal team attach <agent-name>\n")

	return nil
}

// launchAgentSession creates a tmux session for one agent with CC in the first window.
func launchAgentSession(sessionName string, tab AgentTab, cfg *config.Config) error {
	claudeCmd := buildClaudeCommand(tab)

	// Build env vars: TTAL_AGENT_NAME + team context (TTAL_TEAM, TASKRC).
	envParts := []string{fmt.Sprintf("TTAL_AGENT_NAME=%s", tab.Name)}
	if team := cfg.TeamName(); team != "default" || os.Getenv("TTAL_TEAM") != "" {
		envParts = append(envParts, fmt.Sprintf("TTAL_TEAM=%s", team))
	}
	if taskrc := cfg.TaskRC(); taskrc != config.DefaultTaskRC() {
		envParts = append(envParts, fmt.Sprintf("TASKRC=%s", taskrc))
	}

	fishCmd := fmt.Sprintf("env %s fish -C '%s'", strings.Join(envParts, " "), claudeCmd)
	if err := tmux.NewSession(sessionName, tab.Name, tab.Path, fishCmd); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Set team env at session level so new windows/panes inherit
	if team := cfg.TeamName(); team != "default" || os.Getenv("TTAL_TEAM") != "" {
		_ = tmux.SetEnv(sessionName, "TTAL_TEAM", team)
	}
	if taskrc := cfg.TaskRC(); taskrc != config.DefaultTaskRC() {
		_ = tmux.SetEnv(sessionName, "TASKRC", taskrc)
	}

	return nil
}

func buildClaudeCommand(tab AgentTab) string {
	cmd := "claude --dangerously-skip-permissions"
	if tab.Model != "" {
		cmd += " --model " + tab.Model
	}
	if hasConversation(tab.Path) {
		cmd += " --continue"
	}
	return cmd
}

// hasConversation checks if Claude Code has a previous conversation for the given path.
// Claude stores conversations as .jsonl files in ~/.claude/projects/<sanitized-path>/.
func hasConversation(workDir string) bool {
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
