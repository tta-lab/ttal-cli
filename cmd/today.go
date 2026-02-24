package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/today"
	"github.com/spf13/cobra"
)

var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Manage today's task focus list",
	Long:  `View, add, and remove tasks from today's focus list using taskwarrior scheduling.`,
	// Skip database initialization — today commands use taskwarrior directly
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var todayListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show today's scheduled tasks",
	Long: `Show pending tasks scheduled for today or earlier, sorted by urgency.

Tasks with a scheduled date on or before today are included — this keeps
overdue items visible.

Example:
  ttal today list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return today.List()
	},
}

var todayCompletedCmd = &cobra.Command{
	Use:   "completed",
	Short: "Show tasks completed today",
	Long: `Show tasks that were completed today.

Example:
  ttal today completed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return today.Completed()
	},
}

var todayAddCmd = &cobra.Command{
	Use:   "add <uuid> [uuid...]",
	Short: "Add tasks to today's focus list",
	Long: `Schedule tasks for today by setting their scheduled date.

Accepts 8-char UUID prefixes or full UUIDs.

Example:
  ttal today add 5b8fd90c
  ttal today add 5b8fd90c 841f4ec8`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return today.Add(args)
	},
}

var todayRemoveCmd = &cobra.Command{
	Use:   "remove <uuid> [uuid...]",
	Short: "Remove tasks from today's focus list",
	Long: `Clear the scheduled date from tasks, removing them from today's list.

Accepts 8-char UUID prefixes or full UUIDs.

Example:
  ttal today remove 5b8fd90c
  ttal today remove 5b8fd90c 841f4ec8`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return today.Remove(args)
	},
}

func init() {
	rootCmd.AddCommand(todayCmd)
	todayCmd.AddCommand(todayListCmd)
	todayCmd.AddCommand(todayCompletedCmd)
	todayCmd.AddCommand(todayAddCmd)
	todayCmd.AddCommand(todayRemoveCmd)
}
