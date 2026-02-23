package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/team"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage agent sessions",
}

var teamStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start per-agent tmux sessions",
	Long: `Creates a separate tmux session for each agent configured in config.toml.
Each session is named "session-<agent>" and contains a Claude Code window + terminal window.

Without --force: skips already-running sessions (only starts missing ones).
With --force: kills and recreates all sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		return team.Start(database.Client, force)
	},
}

var teamAttachCmd = &cobra.Command{
	Use:   "attach <agent-name>",
	Short: "Attach to an agent's tmux session",
	Long: `Attach the current terminal to an agent's tmux session.

Equivalent to: tmux attach-session -t session-<agent-name>

Example:
  ttal team attach kestrel
  ttal team attach yuki`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return team.Attach(args[0])
	},
}

var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent sessions and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return team.List()
	},
}

var teamStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop all agent tmux sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
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
