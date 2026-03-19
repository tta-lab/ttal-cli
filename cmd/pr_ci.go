package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pr"
)

const (
	ciStateFailure = "failure"
	ciStateError   = "error"
	ciStatePending = "pending"
	ciIconCheck    = "✓"
)

var prCIShowLog bool

var prCICmd = &cobra.Command{
	Use:   "ci",
	Short: "Check CI status for the current PR",
	Long: `Query CI check status for the current PR's HEAD commit.

By default, shows a summary of all checks with pass/fail/pending status.
Use --log to include failure details and log tails for failed jobs.

Works with both GitHub Actions and Woodpecker CI (auto-detected).

Examples:
  ttal pr ci          # show CI status summary
  ttal pr ci --log    # show status + failure logs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		// Resolve HEAD SHA — prefer PR's head SHA from API, fall back to local git
		sha, err := resolveCISHA(ctx)
		if err != nil {
			return err
		}

		// Get combined CI status via daemon
		statusResp, err := daemon.PRGetCombinedStatus(daemon.PRGetCombinedStatusRequest{
			ProviderType: string(ctx.Info.Provider),
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			SHA:          sha,
		})
		if err != nil {
			return fmt.Errorf("failed to get CI status: %w", err)
		}

		printDaemonCIStatus(statusResp, sha)

		if prCIShowLog && hasDaemonCIFailures(statusResp) {
			fmt.Println()
			if err := printDaemonFailureLogs(ctx, sha); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: could not fetch failure logs: %v\n", err)
			}
		}

		return nil
	},
}

// resolveCISHA gets the SHA to check. Prefers the PR's head SHA (from API),
// falls back to local git HEAD.
func resolveCISHA(ctx *pr.Context) (string, error) {
	if ctx.Task.PRID != "" {
		idx, err := pr.PRIndex(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: [pr ci] could not resolve PR index: %v — falling back to local HEAD\n", err)
		} else {
			resp, err := daemon.PRGetPR(daemon.PRGetPRRequest{
				ProviderType: string(ctx.Info.Provider),
				Owner:        ctx.Owner,
				Repo:         ctx.Repo,
				Index:        idx,
			})
			if err == nil && resp.HeadSHA != "" {
				return resp.HeadSHA, nil
			}
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"warning: [pr ci] could not fetch PR #%d from daemon: %v — falling back to local HEAD\n", idx, err)
			}
		}
	}

	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("cannot determine HEAD SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func printDaemonCIStatus(resp daemon.PRCIStatusResponse, sha string) {
	shortSHA := sha
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	fmt.Printf("CI Status for %s: %s\n", shortSHA, formatDaemonCIState(resp.State))

	if len(resp.Statuses) == 0 {
		fmt.Println("  No checks found.")
		return
	}

	for _, s := range resp.Statuses {
		icon := daemonCIStateIcon(s.State)
		fmt.Printf("  %s %s", icon, s.Context)
		if s.Description != "" && s.Description != s.State {
			fmt.Printf(" — %s", s.Description)
		}
		fmt.Println()
	}
}

func printDaemonFailureLogs(ctx *pr.Context, sha string) error {
	detailResp, err := daemon.PRGetCIFailureDetails(daemon.PRGetCIFailureDetailsRequest{
		ProviderType: string(ctx.Info.Provider),
		Owner:        ctx.Owner,
		Repo:         ctx.Repo,
		SHA:          sha,
	})
	if err != nil {
		return err
	}

	if len(detailResp.Details) == 0 {
		fmt.Println("No failure details available.")
		return nil
	}

	fmt.Println("Failure Details:")
	for _, f := range detailResp.Details {
		fmt.Printf("\n  Workflow: %s\n  Job: %s\n", f.WorkflowName, f.JobName)
		if f.HTMLURL != "" {
			fmt.Printf("  URL: %s\n", f.HTMLURL)
		}
		if f.LogTail != "" {
			fmt.Println("  Log tail:")
			for _, line := range strings.Split(f.LogTail, "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
	}
	return nil
}

func formatDaemonCIState(state string) string {
	switch state {
	case "success":
		return "passed"
	case ciStateFailure:
		return "failed"
	case ciStateError:
		return ciStateError
	case ciStatePending:
		return ciStatePending
	default:
		return state
	}
}

func daemonCIStateIcon(state string) string {
	switch state {
	case "success":
		return ciIconCheck
	case ciStateFailure, ciStateError:
		return "✗"
	case ciStatePending:
		return "·"
	default:
		return "?"
	}
}

func hasDaemonCIFailures(resp daemon.PRCIStatusResponse) bool {
	return resp.State == ciStateFailure || resp.State == ciStateError
}
