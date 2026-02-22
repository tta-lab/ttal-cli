package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/pr"
	"codeberg.org/clawteam/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var prTaskUUID string

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests for the current worker task",
	Long: `Create, modify, and comment on Forgejo pull requests.

Context is auto-resolved from ZELLIJ_SESSION_NAME (task UUID prefix).
Use --task to override with an explicit task UUID.

Environment:
  FORGEJO_URL    Forgejo instance URL (default: https://git.guion.io)
  FORGEJO_TOKEN  API token for authentication`,
	// Skip database initialization
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var prCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a PR from the current worker branch",
	Long: `Creates a Forgejo PR using the task's branch and project path.
Stores the PR index in the task's pr_id UDA for future commands.

Examples:
  ttal pr create "feat: add user authentication"
  ttal pr create "fix: resolve timeout bug" --body "Fixes #42"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext(prTaskUUID)
		if err != nil {
			return err
		}

		title := strings.Join(args, " ")
		body, _ := cmd.Flags().GetString("body")

		result, err := pr.Create(ctx, title, body)
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d created: %s\n", result.Index, result.HTMLURL)
		fmt.Printf("  %s → %s\n", result.Head.Name, result.Base.Name)
		return nil
	},
}

var prModifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "Update the PR title or body",
	Long: `Updates the PR associated with the current task.

Examples:
  ttal pr modify --title "new title"
  ttal pr modify --body "updated description"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext(prTaskUUID)
		if err != nil {
			return err
		}

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")

		if title == "" && body == "" {
			return fmt.Errorf("specify --title, --body, or both")
		}

		result, err := pr.Modify(ctx, title, body)
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d updated: %s\n", result.Index, result.HTMLURL)
		return nil
	},
}

var prMergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Squash-merge the PR",
	Long: `Squash-merges the PR associated with the current task.
Fails with a clear error if the PR is not mergeable (conflicts, failing checks).

Examples:
  ttal pr merge
  ttal pr merge --keep-branch`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext(prTaskUUID)
		if err != nil {
			return err
		}

		keepBranch, _ := cmd.Flags().GetBool("keep-branch")

		// Check for uncommitted changes before merging — clean worktree
		// ensures the daemon cleanup can remove the worktree without issues
		if out, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
			if strings.TrimSpace(string(out)) != "" {
				return fmt.Errorf("worktree has uncommitted changes — commit or stash before merging")
			}
		}

		if err := pr.Merge(ctx, !keepBranch); err != nil {
			return err
		}

		fmt.Printf("PR #%s merged (squash)\n", ctx.Task.PRID)
		if !keepBranch {
			fmt.Println("  Branch deleted")
		}

		// Fire-and-forget: request daemon cleanup (session + worktree + task done)
		if ctx.Task.Branch != "" {
			if err := worker.RequestCleanup(ctx.Task.SessionID(), ctx.Task.UUID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup request failed: %v\n", err)
			} else {
				fmt.Println("  Cleanup requested (daemon will close session + worktree)")
			}
		}

		return nil
	},
}

var prCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage PR comments",
}

var prCommentCreateCmd = &cobra.Command{
	Use:   "create <body>",
	Short: "Add a comment to the PR",
	Long: `Adds a comment to the PR associated with the current task.

Examples:
  ttal pr comment create "LGTM, ready to merge"
  ttal pr comment create "Please fix the error handling in auth.go"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext(prTaskUUID)
		if err != nil {
			return err
		}

		body := strings.Join(args, " ")
		comment, err := pr.CommentCreate(ctx, body)
		if err != nil {
			return err
		}

		fmt.Printf("Comment added to PR: %s\n", comment.HTMLURL)
		return nil
	},
}

var prCommentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on the PR",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext(prTaskUUID)
		if err != nil {
			return err
		}

		comments, err := pr.CommentList(ctx)
		if err != nil {
			return err
		}

		if len(comments) == 0 {
			fmt.Println("No comments on this PR.")
			return nil
		}

		for _, c := range comments {
			fmt.Printf("--- %s (%s) ---\n%s\n\n", c.Poster.UserName, c.Created.Format("2006-01-02 15:04"), c.Body)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(prCmd)

	prCmd.PersistentFlags().StringVar(&prTaskUUID, "task", "", "Task UUID (auto-resolved from zellij session if omitted)")

	prCreateCmd.Flags().String("body", "", "PR body/description")
	prModifyCmd.Flags().String("title", "", "New PR title")
	prModifyCmd.Flags().String("body", "", "New PR body")

	prMergeCmd.Flags().Bool("keep-branch", false, "Don't delete the branch after merge")

	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prModifyCmd)
	prCmd.AddCommand(prMergeCmd)
	prCmd.AddCommand(prCommentCmd)

	prCommentCmd.AddCommand(prCommentCreateCmd)
	prCommentCmd.AddCommand(prCommentListCmd)
}
