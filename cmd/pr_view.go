package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
)

type prViewResult struct {
	Index   int64                      `json:"index"`
	Title   string                     `json:"title"`
	State   string                     `json:"state"`
	Merged  bool                       `json:"merged"`
	HTMLURL string                     `json:"html_url"`
	Branch  string                     `json:"branch"`
	Base    string                     `json:"base"`
	Body    string                     `json:"body"`
	CI      *daemon.PRCIStatusResponse `json:"ci,omitempty"`
	HeadSHA string                     `json:"head_sha,omitempty"`
}

var viewJSON bool

var prViewCmd = &cobra.Command{
	Use:     "view",
	Aliases: []string{"list"},
	Short:   "View PR details for the current branch",
	Long: `Resolves the PR for the current branch and shows title, description,
status, and CI summary.

Works from any directory inside a git repo — no task context needed.

For CI failure details and log tails, use: ttal pr log

Examples:
  ttal pr view
  ttal pr list    # alias for same command`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		branch := currentBranchFn(ctx.Task.UUID, ctx.Task.Project, workDir)
		if branch == "" {
			return fmt.Errorf("cannot determine current branch")
		}

		defaultBranch := ctx.Info.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = defaultBranchName
		}

		headSHA := ""
		if ctx.Info.Provider == gitprovider.ProviderGitHub {
			var err error
			headSHA, err = currentBranchHeadSHAFn(workDir, branch)
			if err != nil {
				return fmt.Errorf("cannot verify local branch %s: %w", branch, err)
			}
			if headSHA == "" {
				return fmt.Errorf("cannot verify local branch %s: resolved empty HEAD SHA", branch)
			}
		}

		// Find PR by current branch (State: "all" so closed/merged PRs are visible)
		resp, err := daemonPRFindFn(daemon.PRFindRequest{
			ProviderType: string(ctx.Info.Provider),
			Host:         ctx.Info.Host,
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Head:         branch,
			HeadSHA:      headSHA,
			Base:         defaultBranch,
			State:        "all",
			ProjectAlias: ctx.Alias,
		})
		if err != nil {
			return fmt.Errorf("find PR for branch %s: %w", branch, err)
		}
		if !resp.OK {
			return fmt.Errorf("no PR found for branch %s (create one with: ttal pr create \"title\")", branch)
		}

		// Get PR details
		prResp, err := daemonPRGetPRFn(daemon.PRGetPRRequest{
			ProviderType: string(ctx.Info.Provider),
			Host:         ctx.Info.Host,
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        resp.PRIndex,
			ProjectAlias: ctx.Alias,
		})
		if err != nil {
			return fmt.Errorf("get PR #%d: %w", resp.PRIndex, err)
		}
		if !prResp.OK {
			return fmt.Errorf("get PR #%d: %s", resp.PRIndex, prResp.Error)
		}

		if viewJSON {
			result := prViewResult{
				Index:   resp.PRIndex,
				Title:   prResp.Title,
				State:   prResp.State,
				Merged:  prResp.Merged,
				HTMLURL: prResp.HTMLURL,
				Branch:  branch,
				Base:    defaultBranch,
				Body:    prResp.Body,
				HeadSHA: prResp.HeadSHA,
			}
			if prResp.HeadSHA != "" {
				statusResp, _ := daemon.PRGetCombinedStatus(daemon.PRGetCombinedStatusRequest{
					ProviderType: string(ctx.Info.Provider),
					Host:         ctx.Info.Host,
					Owner:        ctx.Owner,
					Repo:         ctx.Repo,
					SHA:          prResp.HeadSHA,
					ProjectAlias: ctx.Alias,
				})
				if statusResp.OK {
					result.CI = &statusResp
				}
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		// Print PR header
		stateLabel := formatPRState(prResp.State, prResp.Merged)
		fmt.Printf("PR #%d  %s  %s\n", resp.PRIndex, prResp.Title, stateLabel)
		if prResp.HTMLURL != "" {
			fmt.Printf("  %s\n", prResp.HTMLURL)
		}
		fmt.Printf("  %s → %s\n", branch, defaultBranch)

		// Print body preview (first 10 lines)
		if prResp.Body != "" {
			bodyLines := strings.Split(prResp.Body, "\n")
			maxPreview := 10
			if len(bodyLines) > maxPreview {
				bodyLines = bodyLines[:maxPreview]
				bodyLines = append(bodyLines, "...")
			}
			fmt.Printf("\n  %s\n", strings.Join(bodyLines, "\n  "))
		}

		// Show CI status if we have a HEAD SHA
		if prResp.HeadSHA != "" {
			fmt.Println()
			statusResp, err := daemon.PRGetCombinedStatus(daemon.PRGetCombinedStatusRequest{
				ProviderType: string(ctx.Info.Provider),
				Host:         ctx.Info.Host,
				Owner:        ctx.Owner,
				Repo:         ctx.Repo,
				SHA:          prResp.HeadSHA,
				ProjectAlias: ctx.Alias,
			})
			if err == nil {
				printDaemonCIStatus(statusResp, prResp.HeadSHA)
			}
		}

		fmt.Println()
		fmt.Println("For CI failure details: ttal pr log")
		return nil
	},
}

func formatPRState(state string, merged bool) string {
	if merged {
		return "[merged]"
	}
	switch strings.ToLower(state) {
	case "open":
		return "[open]"
	case "closed":
		return "[closed]"
	default:
		return "[" + state + "]"
	}
}

func init() {
	prCmd.AddCommand(prViewCmd)
	prViewCmd.Flags().BoolVar(&viewJSON, "json", false, "Output as JSON")
}
