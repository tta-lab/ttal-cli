package cmd

import (
	"github.com/guion-opensource/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var workerHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Taskwarrior hook handlers",
	Long:  `Commands invoked by taskwarrior on-modify hooks to handle worker lifecycle events.`,
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

var workerHookOnStartCmd = &cobra.Command{
	Use:   "on-start",
	Short: "Handle task start event",
	Long: `Handle task start (+ACTIVE) event from taskwarrior on-modify hook.

Reads two JSON lines from stdin (original and modified task).
Outputs the modified task JSON to stdout (required by taskwarrior).

Routes to matching agent by tag overlap, or defaults to worker-lifecycle.

This command always exits 0 to avoid blocking taskwarrior.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnStart()
	},
}

var workerHookOnCompleteCmd = &cobra.Command{
	Use:   "on-complete",
	Short: "Handle task completion event",
	Long: `Handle task completion event from taskwarrior on-modify hook.

Reads two JSON lines from stdin (original and modified task).
Outputs the modified task JSON to stdout (required by taskwarrior).

For worker tasks (with session_name UDA):
- Calls worker close directly (no subprocess)
- Auto-cleans if PR merged + worktree clean
- Notifies agent if manual decision needed or on error

This command always exits 0 to avoid blocking taskwarrior.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnComplete()
	},
}

func init() {
	workerHookCmd.AddCommand(workerHookOnModifyCmd)
	workerHookCmd.AddCommand(workerHookOnStartCmd)
	workerHookCmd.AddCommand(workerHookOnCompleteCmd)
}
