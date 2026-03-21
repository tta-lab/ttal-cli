package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/planreview"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var planReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Spawn a plan reviewer, or re-review if already running",
	Long: `Spawn a plan-reviewer tmux window for the active task, or trigger re-review if
the window already exists. The task UUID is auto-resolved from session context.

Examples:
  ttal plan review`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid, err := resolveCurrentTask()
		if err != nil {
			return err
		}

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

		reviewerName := resolvePlanReviewerName(uuid)

		fmt.Println("Spawning plan reviewer...")
		return planreview.SpawnPlanReviewer(sessionName, uuid, reviewerName, cfg)
	},
}

// resolvePlanReviewerName resolves the reviewer agent name from pipeline config.
// Tries designer then fixer assignee roles; falls back to "plan-review-lead".
func resolvePlanReviewerName(taskUUID string) string {
	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return "plan-review-lead"
	}
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		return "plan-review-lead"
	}
	for _, role := range []string{"designer", "fixer"} {
		if name := pipelineCfg.ReviewerForStage(task.Tags, role); name != "" {
			return name
		}
	}
	return "plan-review-lead"
}

func init() {
	planCmd.AddCommand(planReviewCmd)
}
