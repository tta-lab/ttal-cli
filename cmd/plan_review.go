package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/planreview"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var planReviewCmd = &cobra.Command{
	Use:   "review <uuid>",
	Short: "Spawn a plan reviewer, or re-review if already running",
	Long: `Spawn a plan-reviewer tmux window for the given task, or trigger re-review if
the window already exists.

Examples:
  ttal plan review abc12345
  ttal plan review abc12345-full-uuid-here`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		if err := taskwarrior.ValidateUUID(uuid); err != nil {
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

		fmt.Println("Spawning plan reviewer...")
		return planreview.SpawnPlanReviewer(sessionName, uuid, cfg)
	},
}

func init() {
	planCmd.AddCommand(planReviewCmd)
}
