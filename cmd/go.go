package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

var taskGoCmd = &cobra.Command{
	Use:   "go <uuid>",
	Short: "Advance a task to the next pipeline stage",
	Long: `Advance a task through its pipeline stages based on pipelines.toml configuration.

This command replaces "ttal task route" and "ttal task execute" with a single,
config-driven operation. The appropriate action (route to agent or spawn worker)
is determined by the task's pipeline stage definition.

Human gate stages block until Telegram approval is received.

Examples:
  ttal task go abc12345
  ttal task go abc12345-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
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
