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
Outputs the task JSON to stdout (required by taskwarrior).

If the task's tags don't match any registered agent, forks a background
enrichment process (claude -p --model haiku) to set project_path and branch UDAs.

This command always exits 0 to avoid blocking taskwarrior.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnAdd()
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

var workerHookEnrichCmd = &cobra.Command{
	Use:    "enrich <uuid>",
	Short:  "Background task enrichment via haiku",
	Long:   `Internal command — called by on-add hook as a detached subprocess. Not for direct use.`,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookEnrich(args[0])
	},
}

var workerHookSpawnWorkerCmd = &cobra.Command{
	Use:    "spawn-worker <uuid> <worker-name> <project-path>",
	Short:  "Background worker spawn",
	Long:   `Internal command — called by on-start hook as a detached subprocess. Not for direct use.`,
	Args:   cobra.ExactArgs(3),
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookSpawnWorker(args[0], args[1], args[2])
	},
}

func init() {
	workerHookCmd.AddCommand(workerHookOnModifyCmd)
	workerHookCmd.AddCommand(workerHookOnAddCmd)
	workerHookCmd.AddCommand(workerHookOnCompleteCmd)
	workerHookCmd.AddCommand(workerHookEnrichCmd)
	workerHookCmd.AddCommand(workerHookSpawnWorkerCmd)
}
