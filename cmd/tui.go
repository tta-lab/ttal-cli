package cmd

import (
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive task management UI",
	Long: `Launch the ttal interactive TUI for browsing, filtering, and acting on tasks.

Provides full ttal integration: execute workers, route to agents, open PRs,
attach to tmux sessions, manage today list, and more.

Key bindings: press ? in the TUI for help.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := tui.NewModel()
		p := tea.NewProgram(m)
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
