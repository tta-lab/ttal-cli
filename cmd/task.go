package cmd

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Taskwarrior task utilities",
	// Skip database initialization — task commands use taskwarrior directly
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var taskGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get formatted task prompt",
	Long: `Export a taskwarrior task and format it as a rich prompt.

Includes description, annotations, and inlined referenced documentation.
Useful for piping to agents or debugging task content.

Accepts 8-char UUID prefixes or full UUIDs.

Examples:
  ttal task get abc12345
  ttal task get abc12345 | ttal send --to eve --stdin`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := taskwarrior.ValidateUUID(args[0]); err != nil {
			return err
		}
		task, err := taskwarrior.ExportTask(args[0])
		if err != nil {
			return err
		}
		fmt.Print(task.FormatPrompt())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
}
