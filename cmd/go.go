package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
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

		sessionName, _ := review.ResolveSessionName()
		workDir, _ := os.Getwd()

		req := daemon.AdvanceRequest{
			TaskUUID:    uuid,
			AgentName:   os.Getenv("TTAL_AGENT_NAME"),
			SessionName: sessionName,
			WorkDir:     workDir,
		}

		resp, err := daemon.AdvanceClient(req)
		if err != nil {
			return fmt.Errorf("advance failed: %w", err)
		}

		switch resp.Status {
		case daemon.AdvanceStatusAdvanced:
			if resp.Agent != "" {
				fmt.Printf("Advanced to stage: %s (%s)\n", resp.Stage, resp.Agent)
			} else if resp.Assignee != "" {
				fmt.Printf("Advanced to stage: %s [%s]\n", resp.Stage, resp.Assignee)
			} else {
				fmt.Printf("Advanced to stage: %s\n", resp.Stage)
			}
		case daemon.AdvanceStatusNoPipeline:
			fmt.Printf("No pipeline: %s\n", resp.Message)
		case daemon.AdvanceStatusNeedsLGTM:
			fmt.Printf("Blocked: %s\n", resp.Message)
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

func init() {
	rootCmd.AddCommand(goCmd)
}
