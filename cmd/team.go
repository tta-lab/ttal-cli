package cmd

import (
	"os"

	"codeberg.org/clawteam/ttal-cli/internal/team"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage agent sessions",
}

var teamStatusCmd = &cobra.Command{
	Use:   "status [team-name]",
	Short: "Show agent session health",
	Long: `Shows the health of all agents in the active team.

Without team name: uses TTAL_TEAM env or default_team from config.
With team name: shows that team's status.

Examples:
  ttal team status              # active team
  ttal team status guion        # specific team`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			_ = os.Setenv("TTAL_TEAM", args[0])
		}
		return team.Status()
	},
}

var teamAttachCmd = &cobra.Command{
	Use:   "attach <agent-name>",
	Short: "Attach to an agent's tmux session",
	Long: `Attach the current terminal to an agent's tmux session.

Supports "team:agent" syntax for explicit team, or bare "agent" for active team.

Examples:
  ttal team attach kestrel         # active team
  ttal team attach guion:mira      # explicit team`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return team.Attach(args[0])
	},
}

var teamListCmd = &cobra.Command{
	Use:   "list [team-name]",
	Short: "List agent sessions and their status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			_ = os.Setenv("TTAL_TEAM", args[0])
		}
		return team.List()
	},
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamStatusCmd)
	teamCmd.AddCommand(teamAttachCmd)
	teamCmd.AddCommand(teamListCmd)
}
