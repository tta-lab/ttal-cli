package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var (
	currentBranchHeadSHAFn = currentBranchHeadSHA
	prResolveContextFn     = pr.ResolveContextWithoutProvider
	currentBranchFn        = worker.CurrentBranch
	daemonPRFindFn         = daemon.PRFind
	daemonPRGetPRFn        = daemon.PRGetPR
	gitPullFn              = daemon.GitPull
)

type pullPRStatus string

const (
	pullPRStatusMerged    pullPRStatus = "merged"
	pullPRStatusUnmerged  pullPRStatus = "unmerged"
	pullPRStatusNotFound  pullPRStatus = "not_found"
	pullPRStatusNoPRCheck pullPRStatus = "no_pr_check"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull current branch or clean up a merged PR branch",
	Long: `Pulls with daemon-held git credentials.

On the default branch, runs a fast-forward pull from origin.
On another branch:
  - if its PR is merged, switches to the default branch, pulls it, and deletes the local and remote branch
  - otherwise, pulls the current branch from origin`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := prResolveContextFn()
		if err != nil {
			return err
		}
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		projectAlias := ctx.Task.Project
		if projectAlias == "" {
			projectAlias = ctx.Alias
		}
		branch := currentBranchFn(ctx.Task.UUID, projectAlias, workDir)
		if branch == "" {
			return fmt.Errorf("cannot determine branch")
		}

		defaultBranch := ctx.Info.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = defaultBranchName
		}

		mode := daemon.GitPullModeBranch
		prStatus := pullPRStatusNoPRCheck
		if branch == defaultBranch {
			mode = daemon.GitPullModeDefault
		} else {
			var err error
			prStatus, err = pullBranchPRStatus(ctx, branch, defaultBranch, workDir)
			if err != nil {
				return err
			}
			if prStatus == pullPRStatusMerged {
				mode = daemon.GitPullModeCleanupMerged
			}
		}

		printPullPlan(cmd, mode, prStatus, branch, defaultBranch)
		resp, err := gitPullFn(daemon.GitPullRequest{
			WorkDir:       workDir,
			Branch:        branch,
			DefaultBranch: defaultBranch,
			ProjectAlias:  ctx.Alias,
			Mode:          mode,
		})
		if err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("pull failed: %s", resp.Error)
		}

		switch resp.Action {
		case daemon.GitPullActionCleanedMergedBranch:
			cmd.Printf("Pulled %s. Deleted %s locally and remotely\n", defaultBranch, branch)
		case daemon.GitPullActionPulledDefault:
			cmd.Printf("Pulled %s\n", defaultBranch)
		default:
			cmd.Printf("Pulled %s\n", branch)
		}
		return nil
	},
}

func pullBranchPRStatus(ctx *pr.Context, branch, defaultBranch, workDir string) (pullPRStatus, error) {
	headSHA, err := currentBranchHeadSHAFn(workDir, branch)
	if err != nil {
		return pullPRStatusNoPRCheck, fmt.Errorf("cannot verify local branch %s: %w", branch, err)
	}
	if headSHA == "" {
		return pullPRStatusNoPRCheck, fmt.Errorf("cannot verify local branch %s: resolved empty HEAD SHA", branch)
	}

	if ctx.Task.PRID != "" {
		index, err := pr.PRIndex(ctx)
		if err != nil {
			return pullPRStatusNoPRCheck, err
		}
		resp, err := daemonPRGetPRFn(daemon.PRGetPRRequest{
			ProviderType: string(ctx.Info.Provider),
			Host:         ctx.Info.Host,
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        index,
			ProjectAlias: ctx.Alias,
		})
		if err != nil {
			return pullPRStatusNoPRCheck, fmt.Errorf("get PR #%d: %w", index, err)
		}
		if !resp.OK {
			return pullPRStatusNoPRCheck, fmt.Errorf("get PR #%d: %s", index, resp.Error)
		}
		if resp.Merged {
			return pullPRStatusMerged, nil
		}
		return pullPRStatusUnmerged, nil
	}

	resp, err := daemonPRFindFn(daemon.PRFindRequest{
		ProviderType: string(ctx.Info.Provider),
		Host:         ctx.Info.Host,
		Owner:        ctx.Owner,
		Repo:         ctx.Repo,
		Head:         branch,
		HeadSHA:      prFindHeadSHA(ctx.Info.Provider, headSHA),
		Base:         defaultBranch,
		State:        "all",
		ProjectAlias: ctx.Alias,
	})
	if err != nil {
		return pullPRStatusNoPRCheck, fmt.Errorf("find PR for branch %s: %w", branch, err)
	}
	if !resp.OK {
		if isPRFindNotFound(resp.Error) {
			return pullPRStatusNotFound, nil
		}
		return pullPRStatusNoPRCheck, fmt.Errorf("find PR for branch %s: %s", branch, resp.Error)
	}
	if resp.Merged {
		return pullPRStatusMerged, nil
	}
	return pullPRStatusUnmerged, nil
}

func prFindHeadSHA(provider gitprovider.ProviderType, headSHA string) string {
	if provider == gitprovider.ProviderGitHub {
		return headSHA
	}
	return ""
}

func printPullPlan(cmd *cobra.Command, mode daemon.GitPullMode, prStatus pullPRStatus, branch, defaultBranch string) {
	switch mode {
	case daemon.GitPullModeDefault:
		cmd.Printf("On default branch %s; pulling latest from origin/%s\n", defaultBranch, defaultBranch)
	case daemon.GitPullModeCleanupMerged:
		cmd.Printf("PR for %s is merged; switching to %s and deleting the branch\n", branch, defaultBranch)
	default:
		switch prStatus {
		case pullPRStatusUnmerged:
			cmd.Printf("PR for %s is not merged; pulling current branch from origin/%s\n", branch, branch)
		case pullPRStatusNotFound:
			cmd.Printf("No PR found for %s; pulling current branch from origin/%s\n", branch, branch)
		default:
			cmd.Printf("Pulling current branch from origin/%s\n", branch)
		}
	}
}

func isPRFindNotFound(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "no pr found") || strings.Contains(lower, "no all pr found") ||
		strings.Contains(lower, "no open pr found")
}

func currentBranchHeadSHA(workDir, branch string) (string, error) {
	args := []string{"rev-parse", branch}
	if workDir != "" {
		args = append([]string{"-C", workDir}, args...)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
