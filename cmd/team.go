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

var teamStartCmd = &cobra.Command{
	Use:   "start [team-name]",
	Short: "Start per-agent tmux sessions",
	Long: `Creates a separate tmux session for each agent configured in config.toml.
Each session is named "ttal-<team>-<agent>" (e.g. "ttal-default-kestrel").

Without team name: uses TTAL_TEAM env or default_team from config.
With team name: starts that team directly.

Without --force: skips already-running sessions (only starts missing ones).
With --force: kills and recreates all sessions.

Examples:
  ttal team start              # start active team
  ttal team start default      # start default team
  ttal team start guion        # start guion team`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			os.Setenv("TTAL_TEAM", args[0])
		}
		force, _ := cmd.Flags().GetBool("force")
		return team.Start(database.Client, force)
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
			os.Setenv("TTAL_TEAM", args[0])
		}
		return team.List()
	},
}

var teamStopCmd = &cobra.Command{
	Use:   "stop [team-name]",
	Short: "Stop all agent tmux sessions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			os.Setenv("TTAL_TEAM", args[0])
		}
		return team.Stop()
	},
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamStartCmd.Flags().Bool("force", false, "Kill and recreate existing sessions")
	teamCmd.AddCommand(teamStartCmd)
	teamCmd.AddCommand(teamAttachCmd)
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamStopCmd)
}
