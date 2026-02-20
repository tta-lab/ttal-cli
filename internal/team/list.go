package team

import (
	"fmt"
	"sort"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// List shows all agent sessions and their status.
func List() error {
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

	fmt.Printf("%-15s %-25s %s\n", "AGENT", "SESSION", "STATUS")
	for _, agentName := range names {
		sessionName := config.AgentSessionName(agentName)
		status := "stopped"
		if tmux.SessionExists(sessionName) {
			status = "running"
		}
		fmt.Printf("%-15s %-25s %s\n", agentName, sessionName, status)
	}

	return nil
}
