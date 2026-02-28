package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var workerHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Taskwarrior hook handlers",
	Long:  `Commands invoked by taskwarrior hooks to handle worker lifecycle events.`,
}

var workerHookOnModifyCmd = &cobra.Command{
	Use:   "on-modify",
	Short: "Handle any on-modify event",
	Long: `Main entry point for taskwarrior on-modify hook.

Reads two JSON lines from stdin, detects the event type (start or complete),
and dispatches to the appropriate handler. For unmatched events, passes through.

This is what the installed hook shim calls. Always exits 0.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnModify()
	},
}

var workerHookOnAddCmd = &cobra.Command{
	Use:   "on-add",
	Short: "Handle task creation event",
	Long: `Handle taskwarrior on-add event.

Reads one JSON line from stdin (the new task).
Enriches the task inline (project_path, branch) if a project is set.
Outputs the task JSON to stdout (required by taskwarrior).

This command always exits 0 to avoid blocking taskwarrior.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnAdd()
	},
}

func init() {
	workerHookCmd.AddCommand(workerHookOnModifyCmd)
	workerHookCmd.AddCommand(workerHookOnAddCmd)
}
