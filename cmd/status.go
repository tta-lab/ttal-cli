package cmd

import (
	"fmt"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/status"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
	"github.com/spf13/cobra"
)

const staleThreshold = 5 * time.Minute

var statusCmd = &cobra.Command{
	Use:   "status [agent]",
	Short: "Show agent context usage and stats",
	Long: `Show live context window usage, model, and cost for running agents.

Without arguments, shows a summary table of all configured agents.
With an agent name, shows detailed stats for that agent.

Data comes from Claude Code's statusline hook, which writes state files
to ~/.ttal/status/ on each assistant message.`,
	// Skip root's DB init — status reads files directly
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return showAgentStatus(args[0])
		}
		return showAllStatus()
	},
}

func showAllStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("%-12s %-8s %-10s %s\n",
		"AGENT", "CTX", "MODEL", "UPDATED")

	team := cfg.TeamName()
	for name := range cfg.Agents {
		s, _ := status.ReadAgent(team, name)
		sessionUp := tmux.SessionExists(config.AgentSessionName(cfg.TeamName(), name))

		if s != nil && !s.IsStale(staleThreshold) {
			age := time.Since(s.UpdatedAt).Truncate(time.Second)
			fmt.Printf("%-12s %-8s %-10s %s ago\n",
				name,
				fmt.Sprintf("%.0f%%", s.ContextUsedPct),
				shortModel(s.ModelName),
				age,
			)
		} else if sessionUp {
			fmt.Printf("%-12s %-8s %-10s %s\n",
				name, "---", "---", "session up, no data")
		} else {
			fmt.Printf("%-12s %-8s %-10s %s\n",
				name, "---", "---", "stopped")
		}
	}
	return nil
}

func showAgentStatus(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := status.ReadAgent(cfg.TeamName(), name)
	if err != nil {
		return err
	}
	sessionUp := tmux.SessionExists(config.AgentSessionName(cfg.TeamName(), name))

	if s == nil || s.IsStale(staleThreshold) {
		if sessionUp {
			fmt.Printf("%s: session running but no status data yet\n", name)
		} else {
			fmt.Printf("%s: not running\n", name)
		}
		return nil
	}

	fmt.Printf("Agent:    %s\n", s.Agent)
	fmt.Printf("Context:  %.0f%% used (%.0f%% remaining)\n", s.ContextUsedPct, s.ContextRemainingPct)
	fmt.Printf("Model:    %s (%s)\n", s.ModelName, s.ModelID)
	fmt.Printf("Session:  %s\n", s.SessionID)
	fmt.Printf("CC:       %s\n", s.CCVersion)
	fmt.Printf("Updated:  %s (%s ago)\n",
		s.UpdatedAt.Local().Format("15:04:05"),
		time.Since(s.UpdatedAt).Truncate(time.Second),
	)
	return nil
}

// shortModel returns a short display name for the model.
func shortModel(name string) string {
	if len(name) > 8 {
		return name[:8]
	}
	return name
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
