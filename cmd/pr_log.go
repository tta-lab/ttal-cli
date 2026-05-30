package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pr"
)

var prLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show CI failure logs for the current PR",
	Long: `Query CI status for the current PR's HEAD commit and show failure details.

Shows a summary of all checks with pass/fail/pending status, plus failure
details and log tails for any failed jobs.

Works with both GitHub Actions and Woodpecker CI (auto-detected).

Examples:
  ttal pr log`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		sha, err := resolveCISHA(ctx)
		if err != nil {
			return err
		}

		statusResp, err := daemon.PRGetCombinedStatus(daemon.PRGetCombinedStatusRequest{
			ProviderType: string(ctx.Info.Provider),
			Host:         ctx.Info.Host,
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			SHA:          sha,
			ProjectAlias: ctx.Task.Project,
		})
		if err != nil {
			return fmt.Errorf("failed to get CI status: %w", err)
		}

		printDaemonCIStatus(statusResp, sha)

		if hasDaemonCIFailures(statusResp) {
			fmt.Println()
			if err := printDaemonFailureLogs(ctx, sha); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: could not fetch failure logs: %v\n", err)
			}
		}

		return nil
	},
}

func init() {
	prCmd.AddCommand(prLogCmd)
}
