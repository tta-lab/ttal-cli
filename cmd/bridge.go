package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/bridge"
	"github.com/spf13/cobra"
)

var bridgeCmd = &cobra.Command{
	Use:    "bridge",
	Short:  "Bridge CC output to Telegram via daemon",
	Hidden: true,
	// Skip database initialization — bridge opens its own DB connection
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return bridge.Run()
	},
}

func init() {
	rootCmd.AddCommand(bridgeCmd)
}
