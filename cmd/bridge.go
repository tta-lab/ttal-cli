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

var bridgeInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the CC Stop hook for Telegram bridging",
	Long: `Add a global Stop hook to ~/.claude/settings.json so that Claude Code
automatically sends assistant responses to Telegram via the daemon.

The hook calls 'ttal bridge' at the end of every CC turn. Non-agent sessions
are silently ignored.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return bridge.Install()
	},
}

var bridgeUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the CC Stop hook for Telegram bridging",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return bridge.Uninstall()
	},
}

func init() {
	rootCmd.AddCommand(bridgeCmd)
	bridgeCmd.AddCommand(bridgeInstallCmd)
	bridgeCmd.AddCommand(bridgeUninstallCmd)
}
