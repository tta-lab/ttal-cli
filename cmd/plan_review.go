package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/planreview"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var planReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Spawn a plan reviewer, or re-review if already running",
	Long: `Spawn a plan-reviewer tmux window for the active task, or trigger re-review if
the window already exists. The task UUID is auto-resolved from TTAL_AGENT_NAME.

Examples:
  ttal plan review`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("TTAL_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("TTAL_AGENT_NAME not set — must be run from an agent session")
		}

		tasks, err := taskwarrior.ExportTasksByFilter("+"+agentName, "+ACTIVE", "status:pending")
		if err != nil {
			return fmt.Errorf("query active tasks for %s: %w", agentName, err)
		}
		if len(tasks) == 0 {
			return fmt.Errorf("no active task found for agent %s — is a task started with +%s tag?", agentName, agentName)
		}
		if len(tasks) > 1 {
			return fmt.Errorf("multiple active tasks found for agent %s — expected exactly one", agentName)
		}
		uuid := tasks[0].UUID

		sessionName, err := review.ResolveSessionName()
		if err != nil {
			return fmt.Errorf("failed to detect tmux session: %w", err)
		}
		if sessionName == "" {
			return fmt.Errorf("must be run inside a tmux session")
		}

		cfg, _ := loadConfigAndCoderRuntime()

		if tmux.WindowExists(sessionName, "plan-review") {
			fmt.Println("Plan reviewer already running, sending re-review request...")
			return planreview.RequestReReview(sessionName, uuid, cfg)
		}

		fmt.Println("Spawning plan reviewer...")
		return planreview.SpawnPlanReviewer(sessionName, uuid, cfg)
	},
}

func init() {
	planCmd.AddCommand(planReviewCmd)
}
