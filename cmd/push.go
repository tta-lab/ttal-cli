package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pr"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push current branch to origin via daemon (no credentials needed in worker)",
	Long: `Pushes the current git branch to origin through the daemon.
The daemon injects credentials — workers don't need tokens in their environment.

Equivalent to "git push -u origin <branch>" but credential-safe for worker sessions.

Examples:
  ttal push`,
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

		branch, err := currentBranch(workDir)
		if err != nil {
			return fmt.Errorf("get current branch: %w", err)
		}

		resp, err := daemon.GitPush(daemon.GitPushRequest{
			WorkDir:      workDir,
			Branch:       branch,
			ProjectAlias: ctx.Alias,
		})
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("push failed: %s", resp.Error)
		}

		fmt.Printf("Pushed %s → origin/%s\n", branch, branch)
		return nil
	},
}

// currentBranch returns the current git branch name for the given working directory.
func currentBranch(workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "git", "-C", workDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("not on a named branch (detached HEAD or empty repo)")
	}
	return branch, nil
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
