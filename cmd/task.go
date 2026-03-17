package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/today"
	"github.com/tta-lab/ttal-cli/internal/tui"
)

const taskStatusCompleted = "completed"

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task orchestration utilities",
}

var taskExecuteCmd = &cobra.Command{
	Use:   "execute <id>",
	Short: "Spawn a worker for a task",
	Long: `Spawn a worker to execute a task. Resolves runtime from task tags
or team's worker_runtime config. Creates a git worktree and tmux session.

Human CLI: prints the resolved project path as a preview, then spawns immediately.
Agent session (TTAL_AGENT_NAME set): sends an approval request to the human via
Telegram/Matrix and blocks until approved.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return spawnWorkerForTask(args[0])
	},
}

var taskHeatmapCmd = &cobra.Command{
	Use:   "heatmap",
	Short: "Show task completion heatmap for the past year",
	Long: `Print a compact GitHub-style heatmap of completed tasks for the past year.

Example:
  ttal task heatmap`,
	RunE: func(cmd *cobra.Command, args []string) error {
		counts, err := today.CompletedCounts()
		if err != nil {
			return fmt.Errorf("loading completed tasks: %w", err)
		}
		fmt.Print(tui.RenderHeatmap(counts, time.Now()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskRouteCmd)
	taskCmd.AddCommand(taskExecuteCmd)
	taskCmd.AddCommand(taskHeatmapCmd)
}
