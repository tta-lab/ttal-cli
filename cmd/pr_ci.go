package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
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
		ctx, err := pr.ResolveContext()
		if err != nil {
			return err
		}

		sha, err := resolveHEADSHA(ctx)
		if err != nil {
			return err
		}

		cs, err := ctx.Provider.GetCombinedStatus(ctx.Owner, ctx.Repo, sha)
		if err != nil {
			return fmt.Errorf("failed to get CI status: %w", err)
		}

		printCIStatus(cs, sha)

		if prCIShowLog && hasCIFailures(cs) {
			fmt.Println()
			if err := printFailureLogs(ctx, sha); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not fetch failure details: %v\n", err)
			}
		}

		return nil
	},
}

// resolveHEADSHA gets the SHA to check. Prefers the PR's head SHA (from API),
// falls back to local git HEAD.
func resolveHEADSHA(ctx *pr.Context) (string, error) {
	if ctx.Task.PRID != "" {
		idx, err := pr.PRIndex(ctx)
		if err == nil {
			fetchedPR, err := ctx.Provider.GetPR(ctx.Owner, ctx.Repo, idx)
			if err == nil && fetchedPR.HeadSHA != "" {
				return fetchedPR.HeadSHA, nil
			}
		}
	}

	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("cannot determine HEAD SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func printCIStatus(cs *gitprovider.CombinedStatus, sha string) {
	shortSHA := sha
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	fmt.Printf("CI Status for %s: %s\n", shortSHA, formatCIState(cs.State))

	if len(cs.Statuses) == 0 {
		fmt.Println("  No checks found.")
		return
	}

	for _, s := range cs.Statuses {
		icon := ciStateIcon(s.State)
		fmt.Printf("  %s %s", icon, s.Context)
		if s.Description != "" && s.Description != s.State {
			fmt.Printf(" — %s", s.Description)
		}
		fmt.Println()
	}
}

func printFailureLogs(ctx *pr.Context, sha string) error {
	failures, err := ctx.Provider.GetCIFailureDetails(ctx.Owner, ctx.Repo, sha)
	if err != nil {
		return err
	}

	if len(failures) == 0 {
		fmt.Println("No failure details available.")
		return nil
	}

	fmt.Println("Failure Details:")
	for _, f := range failures {
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

func formatCIState(state string) string {
	switch state {
	case gitprovider.StateSuccess:
		return "passed"
	case gitprovider.StateFailure:
		return "failed"
	case gitprovider.StateError:
		return "error"
	case gitprovider.StatePending:
		return "pending"
	default:
		return state
	}
}

func ciStateIcon(state string) string {
	switch state {
	case gitprovider.StateSuccess:
		return "✓"
	case gitprovider.StateFailure, gitprovider.StateError:
		return "✗"
	case gitprovider.StatePending:
		return "·"
	default:
		return "?"
	}
}

func hasCIFailures(cs *gitprovider.CombinedStatus) bool {
	return cs.State == gitprovider.StateFailure || cs.State == gitprovider.StateError
}
