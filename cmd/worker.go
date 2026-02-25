package cmd

import (
	"errors"
	"os"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var (
	spawnName          string
	spawnProject       string
	spawnTask          string
	spawnWorktree      bool
	spawnForce         bool
	spawnYolo          bool
	closeForce         bool
	gatekeeperTaskFile string
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Manage coding agent workers",
	Long:  `Spawn, list, and close coding agent workers running in isolated tmux sessions.`,
	// Skip database initialization — worker commands don't need ttal's DB
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var workerInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install taskwarrior hook",
	Long: `Set up the taskwarrior on-modify hook for worker lifecycle management.

Safe to re-run — updates existing installations.

Worker cleanup after PR merge is handled by the ttal daemon.
Run 'ttal daemon install' to set up the daemon.

Example:
  make install          # build ttal binary
  ttal worker install   # set up taskwarrior hook
  ttal daemon install   # set up completion daemon`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return worker.Install()
	},
}

var workerUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove taskwarrior hook",
	Long: `Remove the taskwarrior hook.
Log files are preserved. To also remove the daemon: ttal daemon uninstall`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return worker.Uninstall()
	},
}

var workerSpawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn a new worker",
	Long: `Spawn a coding agent worker in an isolated tmux session.

Creates a git worktree, launches a tmux session with the selected runtime,
and tracks the worker in taskwarrior.

Task tags control behavior:
  +brainstorm  Use brainstorming skill before implementation
  +sonnet      Use sonnet model instead of opus (Claude Code only)
  +opencode    Use OpenCode runtime instead of Claude Code
  +codex       Use Codex runtime instead of Claude Code

Runtime selection:
  --runtime claude-code  (default) Use Claude Code
  --runtime opencode     Use OpenCode
  --runtime codex        Use Codex

Example:
  ttal worker spawn --name fix-auth --project ~/code/myapp --task <uuid>
  ttal worker spawn --name fix-auth --project ~/code/myapp --task <uuid> --runtime opencode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		runtimeStr, _ := cmd.Flags().GetString("runtime")
		rt, err := runtime.Parse(runtimeStr)
		if err != nil {
			return err
		}
		return worker.Spawn(worker.SpawnConfig{
			Name:     spawnName,
			Project:  spawnProject,
			TaskUUID: spawnTask,
			Worktree: spawnWorktree,
			Force:    spawnForce,
			Yolo:     spawnYolo,
			Runtime:  rt,
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

func init() {
	rootCmd.AddCommand(workerCmd)

	workerCmd.AddCommand(workerInstallCmd)
	workerCmd.AddCommand(workerUninstallCmd)
	workerCmd.AddCommand(workerSpawnCmd)
	workerCmd.AddCommand(workerListCmd)
	workerCmd.AddCommand(workerCloseCmd)
	workerCmd.AddCommand(workerHookCmd)
	workerCmd.AddCommand(workerGatekeeperCmd)

	// Spawn flags
	workerSpawnCmd.Flags().StringVar(&spawnName, "name", "", "Worker name (required)")
	workerSpawnCmd.Flags().StringVar(&spawnProject, "project", "", "Project directory (required)")
	workerSpawnCmd.Flags().StringVar(&spawnTask, "task", "", "Task UUID (required)")
	workerSpawnCmd.Flags().BoolVar(&spawnWorktree, "worktree", true, "Create git worktree")
	workerSpawnCmd.Flags().BoolVar(&spawnForce, "force", false, "Force respawn (close existing session)")
	workerSpawnCmd.Flags().BoolVar(&spawnYolo, "yolo", true, "Skip permissions prompts")
	workerSpawnCmd.Flags().String("runtime", "claude-code", "Coding agent runtime (claude-code, opencode, codex)")

	_ = workerSpawnCmd.MarkFlagRequired("name")
	_ = workerSpawnCmd.MarkFlagRequired("project")
	_ = workerSpawnCmd.MarkFlagRequired("task")

	// Close flags
	workerCloseCmd.Flags().BoolVar(&closeForce, "force", false, "Force cleanup regardless of PR status")

	// Gatekeeper flags
	workerGatekeeperCmd.Flags().StringVar(
		&gatekeeperTaskFile, "task-file", "", "Task file to read and append to command",
	)

}

func init() {
	// Register --no-worktree and --no-yolo as inverse flags
	workerSpawnCmd.Flags().Lookup("worktree").NoOptDefVal = "true"
	workerSpawnCmd.Flags().Lookup("yolo").NoOptDefVal = "true"
}
