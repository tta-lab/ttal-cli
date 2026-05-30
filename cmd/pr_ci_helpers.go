package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pr"
)

const (
	ciStateFailure = "failure"
	ciStateError   = "error"
	ciStatePending = "pending"
	ciIconCheck    = "✓"
)

// resolveCISHA gets the SHA to check. Prefers the PR's head SHA (from API),
// falls back to local git HEAD.
func resolveCISHA(ctx *pr.Context) (string, error) {
	if ctx.Task.PRID != "" {
		idx, err := pr.PRIndex(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: [pr log] could not resolve PR index: %v — falling back to local HEAD\n", err)
		} else {
			resp, err := daemon.PRGetPR(daemon.PRGetPRRequest{
				ProviderType: string(ctx.Info.Provider),
				Host:         ctx.Info.Host,
				Owner:        ctx.Owner,
				Repo:         ctx.Repo,
				Index:        idx,
				ProjectAlias: ctx.Task.Project,
			})
			if err == nil && resp.HeadSHA != "" {
				return resp.HeadSHA, nil
			}
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"warning: [pr log] could not fetch PR #%d from daemon: %v — falling back to local HEAD\n", idx, err)
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
		Host:         ctx.Info.Host,
		Owner:        ctx.Owner,
		Repo:         ctx.Repo,
		SHA:          sha,
		ProjectAlias: ctx.Task.Project,
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
