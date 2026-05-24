package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pr"
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
		if branch == defaultBranch {
			mode = daemon.GitPullModeDefault
		} else {
			merged, err := pullBranchHasMergedPR(ctx, branch, defaultBranch)
			if err != nil {
				return err
			}
			if merged {
				mode = daemon.GitPullModeCleanupMerged
			}
		}

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

func pullBranchHasMergedPR(ctx *pr.Context, branch, defaultBranch string) (bool, error) {
	if ctx.Task.PRID != "" {
		index, err := pr.PRIndex(ctx)
		if err != nil {
			return false, err
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
			return false, fmt.Errorf("get PR #%d: %w", index, err)
		}
		if !resp.OK {
			return false, fmt.Errorf("get PR #%d: %s", index, resp.Error)
		}
		return resp.Merged, nil
	}

	resp, err := daemonPRFindFn(daemon.PRFindRequest{
		ProviderType: string(ctx.Info.Provider),
		Host:         ctx.Info.Host,
		Owner:        ctx.Owner,
		Repo:         ctx.Repo,
		Head:         branch,
		Base:         defaultBranch,
		State:        "all",
		ProjectAlias: ctx.Alias,
	})
	if err != nil {
		return false, nil
	}
	if !resp.OK {
		return false, nil
	}
	return resp.Merged, nil
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
