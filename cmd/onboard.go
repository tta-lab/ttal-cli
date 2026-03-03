package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/onboard"
)

var (
	onboardWorkspace string
	onboardScaffold  string
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "First-time setup — install prerequisites, scaffold workspace, configure agents",
	Long: `Run once on a new machine to set up the full ttal environment:

  1. Install prerequisites via brew (tmux, taskwarrior, zellij, ffmpeg)
  2. Set up workspace from a scaffold template (basic, full-markdown, full-flicknote)
  3. Set up taskwarrior UDAs and config template
  4. Register discovered agents in the database
  5. Install daemon launchd plist and taskwarrior hooks

Safe to re-run — skips steps that are already done.

Example:
  ttal onboard
  ttal onboard --scaffold full-markdown --workspace ~/my-agents`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return onboard.Run(onboardWorkspace, onboardScaffold)
	},
}

func init() {
	onboardCmd.Flags().StringVar(&onboardWorkspace, "workspace", "~/ttal-workspace",
		"Where to set up the agent workspace")
	onboardCmd.Flags().StringVar(&onboardScaffold, "scaffold", "basic",
		"Which scaffold to use (basic, full-markdown, full-flicknote)")
	rootCmd.AddCommand(onboardCmd)
}
