package cmd

import (
	"errors"
	"os"

	"github.com/guion-opensource/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var (
	spawnName       string
	spawnProject    string
	spawnTask       string
	spawnSession    string
	spawnWorktree   bool
	spawnForce      bool
	spawnYolo       bool
	spawnBrainstorm bool
	closeForce      bool
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Manage Claude Code workers",
	Long:  `Spawn, list, close, and poll Claude Code workers running in isolated zellij sessions.`,
	// Skip database initialization — worker commands don't need ttal's DB
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var workerSpawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn a new worker",
	Long: `Spawn a Claude Code worker in an isolated zellij session.

Creates a git worktree, launches a zellij session with Claude Code,
and tracks the worker in taskwarrior.

Example:
  ttal worker spawn --name fix-auth --project ~/code/myapp --task <uuid>
  ttal worker spawn --name refactor --project . --task <uuid> --brainstorm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return worker.Spawn(worker.SpawnConfig{
			Name:       spawnName,
			Project:    spawnProject,
			TaskUUID:   spawnTask,
			Session:    spawnSession,
			Worktree:   spawnWorktree,
			Force:      spawnForce,
			Yolo:       spawnYolo,
			Brainstorm: spawnBrainstorm,
		})
	},
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

var workerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active workers",
	Long: `List all active Claude Code workers with their status.

Shows a table of active workers categorized by PR status:
  RUNNING  - Worker is active, no PR created yet
  WITH_PR  - Worker has created a PR (not yet merged)
  CLEANUP  - PR is merged, worker needs cleanup

Example:
  ttal worker list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return worker.List()
	},
}

var workerPollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll for completed workers",
	Long: `Poll for merged PRs and auto-complete worker tasks.

Checks each active worker task for a merged PR. If merged,
marks the task as done (triggers existing cleanup hooks).
Also cleans up stale temp files older than 24 hours.

Intended to run periodically via launchd/cron:
  ttal worker poll`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return worker.Poll()
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)

	workerCmd.AddCommand(workerSpawnCmd)
	workerCmd.AddCommand(workerListCmd)
	workerCmd.AddCommand(workerCloseCmd)
	workerCmd.AddCommand(workerPollCmd)
	workerCmd.AddCommand(workerHookCmd)

	// Spawn flags
	workerSpawnCmd.Flags().StringVar(&spawnName, "name", "", "Worker name (required)")
	workerSpawnCmd.Flags().StringVar(&spawnProject, "project", "", "Project directory (required)")
	workerSpawnCmd.Flags().StringVar(&spawnTask, "task", "", "Task UUID (required)")
	workerSpawnCmd.Flags().StringVar(&spawnSession, "session", "", "Custom session name (default: random 8-char ID)")
	workerSpawnCmd.Flags().BoolVar(&spawnWorktree, "worktree", true, "Create git worktree")
	workerSpawnCmd.Flags().BoolVar(&spawnForce, "force", false, "Force respawn (close existing session)")
	workerSpawnCmd.Flags().BoolVar(&spawnYolo, "yolo", true, "Skip permissions prompts")
	workerSpawnCmd.Flags().BoolVar(&spawnBrainstorm, "brainstorm", false, "Use brainstorming skill")

	_ = workerSpawnCmd.MarkFlagRequired("name")
	_ = workerSpawnCmd.MarkFlagRequired("project")
	_ = workerSpawnCmd.MarkFlagRequired("task")

	// Close flags
	workerCloseCmd.Flags().BoolVar(&closeForce, "force", false, "Force cleanup regardless of PR status")
}

func init() {
	// Register --no-worktree and --no-yolo as inverse flags
	workerSpawnCmd.Flags().Lookup("worktree").NoOptDefVal = "true"
	workerSpawnCmd.Flags().Lookup("yolo").NoOptDefVal = "true"
}
