package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/planreview"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var goCmd = &cobra.Command{
	Use:   "go [uuid]",
	Short: "Advance a task to the next pipeline stage",
	Long: `Advance a task through its pipeline stages based on pipelines.toml configuration.

This command replaces "ttal task route" and "ttal task execute" with a single,
config-driven operation. The appropriate action (route to agent or spawn worker)
is determined by the task's pipeline stage definition.

Human gate stages block until Telegram approval is received.

If no UUID is provided, resolves the current task from:
  - TTAL_JOB_ID (worker sessions)
  - TTAL_AGENT_NAME (manager sessions — active task with matching tag)

Examples:
  ttal go abc12345
  ttal go abc12345-1234-1234-1234-123456789abc
  ttal go`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var uuid string
		if len(args) > 0 {
			uuid = args[0]
		} else {
			resolved, err := resolveCurrentTask()
			if err != nil {
				return fmt.Errorf("no UUID provided and auto-resolve failed: %w", err)
			}
			uuid = resolved
		}
		if err := taskwarrior.ValidateUUID(uuid); err != nil {
			return err
		}

		req := daemon.AdvanceRequest{
			TaskUUID:  uuid,
			AgentName: os.Getenv("TTAL_AGENT_NAME"),
			Team:      os.Getenv("TTAL_TEAM"),
		}

		resp, err := daemon.AdvanceClient(req)
		if err != nil {
			return fmt.Errorf("advance failed: %w", err)
		}

		switch resp.Status {
		case daemon.AdvanceStatusAdvanced:
			fmt.Printf("Advanced to stage: %s\n", resp.Stage)
		case daemon.AdvanceStatusNoPipeline:
			fmt.Printf("No pipeline: %s\n", resp.Message)
		case daemon.AdvanceStatusNeedsLGTM:
			fmt.Printf("Blocked: %s\n", resp.Message)
			if resp.Reviewer != "" {
				if err := spawnOrRetriggerReviewer(uuid, resp.Reviewer, resp.Assignee); err != nil {
					fmt.Fprintf(os.Stderr, "warning: reviewer spawn failed: %v\n", err)
				}
			}
		case daemon.AdvanceStatusRejected:
			return fmt.Errorf("rejected: %s", resp.Message)
		case daemon.AdvanceStatusComplete:
			fmt.Printf("Pipeline complete: %s\n", resp.Message)
		case daemon.AdvanceStatusError:
			return fmt.Errorf("error: %s", resp.Message)
		default:
			fmt.Printf("Status: %s — %s\n", resp.Status, resp.Message)
		}
		return nil
	},
}

// spawnOrRetriggerReviewer spawns or re-triggers a reviewer based on the stage assignee.
// For worker stages it spawns a PR reviewer; for other stages it spawns a plan reviewer.
func spawnOrRetriggerReviewer(taskUUID, reviewerAgent, assignee string) error {
	sessionName, err := review.ResolveSessionName()
	if err != nil {
		return fmt.Errorf("resolve tmux session: %w", err)
	}
	if sessionName == "" {
		fmt.Println("Not in a tmux session — run from a tmux session to auto-spawn reviewer")
		return nil
	}
	cfg, _ := loadConfigAndCoderRuntime()

	if assignee == "worker" {
		if tmux.WindowExists(sessionName, "review") {
			fmt.Println("Reviewer already running, sending re-review request...")
			return review.RequestReReview(sessionName, false, "", cfg)
		}
		fmt.Println("Spawning PR reviewer...")
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return fmt.Errorf("resolve PR context: %w", err)
		}
		return review.SpawnReviewer(sessionName, ctx, reviewerAgent, cfg)
	}

	if tmux.WindowExists(sessionName, "plan-review") {
		fmt.Println("Plan reviewer already running, sending re-review request...")
		return planreview.RequestReReview(sessionName, "", cfg)
	}
	fmt.Println("Spawning plan reviewer...")
	return planreview.SpawnPlanReviewer(sessionName, taskUUID, reviewerAgent, cfg)
}

func init() {
	rootCmd.AddCommand(goCmd)
}
