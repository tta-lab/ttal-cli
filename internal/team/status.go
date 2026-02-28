package team

import (
	"fmt"
	"sort"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// Status prints the health of all agents in the active team.
func Status() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	running, _, _ := daemon.IsRunning()
	if !running {
		fmt.Printf("Team: %s (daemon not running)\n", cfg.TeamName())
		return nil
	}

	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("Team: %s\n", cfg.TeamName())
	total := len(names)
	active := 0
	for _, agentName := range names {
		rt := cfg.AgentRuntimeFor(agentName)
		sessionName := config.AgentSessionName(cfg.TeamName(), agentName)
		switch rt {
		case runtime.ClaudeCode:
			if tmux.SessionExists(sessionName) {
				fmt.Printf("  ✓ %s (claude-code, session: %s)\n", agentName, sessionName)
				active++
			} else {
				fmt.Printf("  ✗ %s (claude-code, not running)\n", agentName)
			}
		case runtime.OpenCode, runtime.Codex:
			port := cfg.Agents[agentName].Port
			if port == 0 {
				fmt.Printf("  ✗ %s (%s, no port configured)\n", agentName, rt)
			} else {
				// Port is configured but we can't verify the adapter is actually healthy
				// from the CLI — the daemon manages adapter lifecycle internally.
				fmt.Printf("  ~ %s (%s, port %d)\n", agentName, rt, port)
				active++
			}
		case runtime.OpenClaw:
			fmt.Printf("  ● %s (openclaw, self-managed)\n", agentName)
			active++
		}
	}
	fmt.Printf("\n%d/%d agents active\n", active, total)
	return nil
}
