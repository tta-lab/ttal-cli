package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/tui"
)

var teamFlag string

var rootCmd = &cobra.Command{
	Use:   "ttal",
	Short: "TTAL - Task & Team Agent Lifecycle Manager",
	Long: `TTAL is a CLI tool for managing projects, agents, workers, tasks, and daily focus.
It provides taskwarrior-like syntax for tag management and agent routing.

Running ttal with no subcommand launches the interactive TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := tui.NewModel()
		p := tea.NewProgram(m)
		_, err := p.Run()
		return err
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if teamFlag != "" {
			os.Setenv("TTAL_TEAM", teamFlag)
		}
		// Load .env as fallback for tokens not already in the environment
		if dotEnv, err := config.LoadDotEnv(); err == nil {
			for k, v := range dotEnv {
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&teamFlag, "team", "", "Team to use (overrides TTAL_TEAM env)")
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

// deleteEntity checks existence, confirms with user, then deletes.
// existFn checks if the entity exists, deleteFn performs the deletion.
func deleteEntity(kind, name string, existFn func() (bool, error), deleteFn func() error) error {
	exists, err := existFn()
	if err != nil {
		return fmt.Errorf("failed to query %s: %w", kind, err)
	}
	if !exists {
		return fmt.Errorf("%s '%s' not found", kind, name)
	}

	if !confirmPrompt(fmt.Sprintf("Permanently delete %s '%s'? [y/N] ", kind, name)) {
		fmt.Println("Aborted.")
		return nil
	}

	if err := deleteFn(); err != nil {
		return fmt.Errorf("failed to delete %s: %w", kind, err)
	}

	fmt.Printf("%s '%s' deleted permanently\n", strings.ToUpper(kind[:1])+kind[1:], name)
	return nil
}
