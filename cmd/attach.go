package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/team"
)

var attachCmd = &cobra.Command{
	Use:   "attach <agent-name>",
	Short: "Attach to an agent's tmux session",
	Long: `Attach the current terminal to an agent's tmux session.

Supports "team:agent" syntax for explicit team, or bare "agent" for active team.

Examples:
  ttal attach kestrel         # active team
  ttal attach guion:mira      # explicit team`,
	// Skip root's DB init — attach doesn't need database
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return team.Attach(args[0])
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
