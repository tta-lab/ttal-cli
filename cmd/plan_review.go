package cmd

import (
	"fmt"
	"os"

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

const defaultPlanReviewerName = "plan-review-lead"

// resolvePlanReviewerName resolves the reviewer agent name from pipeline config.
// Tries designer then fixer assignee roles; falls back to defaultPlanReviewerName.
func resolvePlanReviewerName(taskUUID string) string {
	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not export task %s — falling back to %s: %v\n", taskUUID, defaultPlanReviewerName, err)
		return defaultPlanReviewerName
	}
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load pipelines.toml — falling back to %s: %v\n", defaultPlanReviewerName, err)
		return defaultPlanReviewerName
	}
	for _, role := range []string{"designer", "fixer"} {
		if name := pipelineCfg.ReviewerForStage(task.Tags, role); name != "" {
			return name
		}
	}
	return defaultPlanReviewerName
}

func init() {
	planCmd.AddCommand(planReviewCmd)
}
