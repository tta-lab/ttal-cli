package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/onboard"
)

var onboardWorkspace string

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "First-time setup — install prerequisites, clone starter, configure agents",
	Long: `Run once on a new machine to set up the full ttal environment:

  1. Install prerequisites via brew (tmux, taskwarrior, zellij, ffmpeg)
  2. Clone ttal-starter template repo with example agents
  3. Set up taskwarrior UDAs and config template
  4. Register discovered agents in the database
  5. Install daemon launchd plist and taskwarrior hooks

Safe to re-run — skips steps that are already done.

Example:
  ttal onboard
  ttal onboard --workspace ~/my-agents`,
	// Skip DB init — onboard creates it implicitly via agent registration
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return onboard.Run(onboardWorkspace)
	},
}

func init() {
	onboardCmd.Flags().StringVar(&onboardWorkspace, "workspace", "~/ttal-workspace", "Where to clone the starter repo")
	rootCmd.AddCommand(onboardCmd)
}
