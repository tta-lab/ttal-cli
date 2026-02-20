package team

import (
	"fmt"
	"sort"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// Stop kills all agent tmux sessions.
func Stop() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Agents) == 0 {
		fmt.Println("No agents configured.")
		return nil
	}

	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sort.Strings(names)

	killed := 0
	for _, agentName := range names {
		sessionName := config.AgentSessionName(agentName)
		if !tmux.SessionExists(sessionName) {
			continue
		}
		if err := tmux.KillSession(sessionName); err != nil {
			fmt.Printf("  ✗ %s: %v\n", agentName, err)
			continue
		}
		fmt.Printf("  ✓ %s stopped\n", agentName)
		killed++
	}

	if killed == 0 {
		fmt.Println("No running sessions to stop.")
	} else {
		fmt.Printf("\nStopped %d session(s).\n", killed)
	}

	return nil
}
