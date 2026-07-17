package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "ttal",
	Short: "TTAL - Task & Team Agent Lifecycle Manager",
	Long: `TTAL is a CLI tool for coordinating agents, workers, messaging, and existing pipelines.

Running ttal with no subcommand launches the interactive TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := tui.NewModel()
		p := tea.NewProgram(m)
		stop, err := tui.StartWatcher(p)
		if err != nil {
			return fmt.Errorf("watcher: %w", err)
		}
		defer stop()
		_, err = p.Run()
		return err
	},
	// PersistentPreRunE removed — .env is loaded only by commands that need it
	// (ttal daemon). Daemon proxies authenticated operations.
}

func init() {
}

func Execute() error {
	return rootCmd.Execute()
}

// confirmPrompt asks the user a yes/no question and returns true if they answer "y".
func confirmPrompt(message string) bool {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}
