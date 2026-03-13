package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var (
	closeForce         bool
	cleanupForce       bool
	gatekeeperTaskFile string
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Manage coding agent workers",
	Long:  `Spawn, list, and close coding agent workers running in isolated tmux sessions.`,
}

var workerCloseCmd = &cobra.Command{
	Use:   "close <session-name>",
	Short: "Close a worker and cleanup",
	Long: `Close a worker and cleanup its session.

Smart mode (default): Auto-cleanup if safe (PR merged + clean worktree).
Force mode (--force): Dump state and cleanup regardless.

Exit codes:
  0 = Cleaned up successfully
  1 = Needs manual decision (unsafe to auto-cleanup)
  2 = Error (worker not found, script error)

Example:
  ttal worker close a7f3d2b9
  ttal worker close a7f3d2b9 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := worker.Close(args[0], closeForce)
		if result != nil {
			worker.PrintResult(result)
		}

		if err != nil {
			if errors.Is(err, worker.ErrNeedsDecision) {
				os.Exit(1)
			}
			os.Exit(2)
		}
		return nil
	},
}

var workerGatekeeperCmd = &cobra.Command{
	Use:    "gatekeeper -- <command> [args...]",
	Short:  "Run a command with deadman's switch (internal)",
	Long:   `Wraps a child process with orphan detection and signal forwarding. Used internally by worker spawn.`,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dashIdx := cmd.ArgsLenAtDash()
		if dashIdx < 0 || dashIdx >= len(args) {
			return errors.New("usage: ttal worker gatekeeper [--task-file FILE] -- <command> [args...]")
		}
		childArgs := args[dashIdx:]
		exitCode := worker.Gatekeeper(worker.GatekeeperConfig{
			TaskFile: gatekeeperTaskFile,
			Command:  childArgs,
		})
		os.Exit(exitCode) // bypass cobra to propagate child's exact exit code
		return nil
	},
}

var workerCleanupCmd = &cobra.Command{
	Use:   "cleanup [file]",
	Short: "Process pending cleanup requests",
	Long: `Manually trigger cleanup processing for pending requests.

Without arguments, processes all pending cleanup files in ~/.ttal/cleanup/.
With a file argument, processes that specific cleanup request.

This is useful when the daemon is not running or a cleanup was missed.

Example:
  ttal worker cleanup                              # process all pending
  ttal worker cleanup ~/.ttal/cleanup/session.json  # process one file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return worker.RunCleanup(args[0], cleanupForce)
		}
		return worker.RunPendingCleanups(cleanupForce)
	},
}

var workerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active workers",
	Long: `List all active Claude Code workers with their status.

Shows a table of active workers categorized by PR status:
  RUNNING  - Worker is active, no PR created yet
  WITH_PR  - Worker has an open PR

Example:
  ttal worker list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return worker.List()
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)

	workerCmd.AddCommand(workerListCmd)
	workerCmd.AddCommand(workerCloseCmd)
	workerCmd.AddCommand(workerCleanupCmd)
	workerCmd.AddCommand(workerHookCmd)
	workerCmd.AddCommand(workerGatekeeperCmd)

	// Close flags
	workerCloseCmd.Flags().BoolVar(&closeForce, "force", false, "Force cleanup regardless of PR status")

	// Cleanup flags
	workerCleanupCmd.Flags().BoolVar(&cleanupForce, "force", false, "Force cleanup regardless of PR status")

	// Gatekeeper flags
	workerGatekeeperCmd.Flags().StringVar(
		&gatekeeperTaskFile, "task-file", "", "Task file to read and append to command",
	)

}
