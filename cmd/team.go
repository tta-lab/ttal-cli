package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/team"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage the team zellij session",
}

var teamStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the team session with a tab per agent",
	Long: `Creates a zellij session (name from daemon.json zellij_session) with one tab
per agent. Each tab runs Claude Code (yolo, --continue) in the agent's registered path.

Agents are read from daemon.json, paths from the ttal database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		return team.Start(database.Client, force)
	},
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamStartCmd.Flags().Bool("force", false, "Kill and recreate existing session")
	teamCmd.AddCommand(teamStartCmd)
}
