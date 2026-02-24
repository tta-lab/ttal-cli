package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/pr"
	"codeberg.org/clawteam/ttal-cli/internal/review"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
	"codeberg.org/clawteam/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests for the current worker task",
	Long: `Create, modify, and comment on pull requests.

Context is auto-resolved from TTAL_JOB_ID (task UUID prefix).
Provider is auto-detected from git remote URL (github.com → GitHub, else → Forgejo).

Environment:
  GITHUB_TOKEN    GitHub API token (required for GitHub repos)
  FORGEJO_URL     Forgejo instance URL (required for Forgejo repos)
  FORGEJO_TOKEN   Forgejo API token (required for Forgejo repos)`,
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
		ctx, err := pr.ResolveContext()
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
		fmt.Printf("  %s → %s\n", result.Head, result.Base)
		fmt.Println()

		// Update ctx with newly created PR index so SpawnReviewer can use it.
		// pr.Create stores it in taskwarrior, but ctx.Task is a stale snapshot.
		ctx.Task.PRID = strconv.FormatInt(result.Index, 10)

		// Auto-spawn reviewer
		sessionName, sessionErr := review.ResolveSessionName()
		if sessionErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to detect tmux session: %v\n", sessionErr)
		} else if sessionName != "" {
			if tmux.WindowExists(sessionName, "review") {
				fmt.Println("  Reviewer already running, sending review request...")
				if err := review.RequestReReview(sessionName, false); err != nil {
					fmt.Fprintf(os.Stderr, "warning: re-review request failed: %v\n", err)
				}
			} else {
				fmt.Println("  Spawning reviewer...")
				if err := review.SpawnReviewer(sessionName, ctx); err != nil {
					fmt.Fprintf(os.Stderr, "warning: auto-spawn reviewer failed: %v\n", err)
				}
			}
		} else {
			fmt.Println("  To request a code review:")
			fmt.Println("    ttal pr review")
		}

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
		ctx, err := pr.ResolveContext()
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
		ctx, err := pr.ResolveContext()
		if err != nil {
			return err
		}

		keepBranch, _ := cmd.Flags().GetBool("keep-branch")

		// Check for uncommitted changes before merging — clean worktree
		// ensures the daemon cleanup can remove the worktree without issues
		statusOut, statusErr := exec.Command("git", "status", "--porcelain").Output()
		if statusErr != nil {
			return fmt.Errorf("failed to check worktree status: %w", statusErr)
		}
		if strings.TrimSpace(string(statusOut)) != "" {
			return fmt.Errorf("worktree has uncommitted changes — commit or stash before merging")
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
			if err := worker.RequestCleanup(ctx.Task.SessionName(), ctx.Task.UUID); err != nil {
				fmt.Fprintf(os.Stderr,
					"warning: cleanup request failed: %v\n  run: ttal worker close %s\n",
					err, ctx.Task.SessionName())
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
		ctx, err := pr.ResolveContext()
		if err != nil {
			return err
		}

		body := strings.Join(args, " ")
		comment, err := pr.CommentCreate(ctx, body)
		if err != nil {
			return err
		}

		fmt.Printf("Comment added to PR: %s\n", comment.HTMLURL)

		// Route based on TTAL_ROLE (set by worker/reviewer spawn).
		// Reviewer → notify coder window. Coder → trigger re-review.
		sessionName, sessionErr := review.ResolveSessionName()
		if sessionErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to detect tmux session: %v\n", sessionErr)
		}
		role := tmux.Role()
		if sessionName != "" && role == "reviewer" {
			// Reviewer posting → notify the coder window
			coderWindow, cwErr := tmux.FirstWindowExcept(sessionName, "review")
			if cwErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to find coder window: %v\n", cwErr)
			}
			if coderWindow != "" {
				// Write review comment to temp file for direct delivery to worker
				reviewFile, fileErr := writeReviewFile(body)
				if fileErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to write review file: %v\n", fileErr)
				}

				mergeHint := " If verdict is LGTM and no remaining issues, merge with: ttal pr merge"
				var notification string
				if reviewFile != "" {
					notification = fmt.Sprintf(
						"/triage PR review posted. Full review at %s"+
							" — read it, assess and fix issues."+
							" Post your triage update with ttal pr comment create when done."+
							mergeHint,
						reviewFile)
				} else {
					notification = "/triage PR reviewed — see PR comments. " +
						"Assess and fix issues, then post your triage update with ttal pr comment create." +
						mergeHint
				}
				if err := tmux.SendKeys(sessionName, coderWindow, notification); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to notify coder window: %v\n", err)
				}
			}
		}

		// Auto-trigger re-review when coder posts a comment (triage done).
		if sessionName != "" && role == "coder" {
			if tmux.WindowExists(sessionName, "review") {
				fmt.Println("  Triggering re-review...")
				if err := review.RequestReReview(sessionName, false); err != nil {
					fmt.Fprintf(os.Stderr, "warning: re-review request failed: %v\n", err)
				}
			} else {
				// Reviewer window gone (crashed or closed) — respawn it
				fmt.Println("  Reviewer not running, spawning...")
				if err := review.SpawnReviewer(sessionName, ctx); err != nil {
					fmt.Fprintf(os.Stderr, "warning: auto-spawn reviewer failed: %v\n", err)
				}
			}
		}

		return nil
	},
}

var (
	reviewForce bool
	reviewFull  bool
)

var prReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Spawn a reviewer to review the current PR",
	Long: `Manually spawn or re-trigger a PR reviewer.

In normal flow, reviews are triggered automatically:
- On PR create: reviewer spawns automatically
- On worker comment: re-review triggers automatically

Use this command when you need to:
- Respawn a crashed reviewer (--force)
- Force a full re-review instead of delta (--full)
- Manually trigger a review in non-standard situations

Examples:
  ttal pr review         # spawn reviewer (or re-review if window exists)
  ttal pr review --full  # force full re-review (not delta)
  ttal pr review --force # kill and respawn reviewer window`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext()
		if err != nil {
			return err
		}

		sessionName, err := review.ResolveSessionName()
		if err != nil {
			return fmt.Errorf("failed to detect tmux session: %w", err)
		}
		if sessionName == "" {
			return fmt.Errorf("must be run inside a tmux session")
		}

		if reviewForce && tmux.WindowExists(sessionName, "review") {
			if err := tmux.KillWindow(sessionName, "review"); err != nil {
				return fmt.Errorf("failed to kill reviewer window: %w", err)
			}
			fmt.Println("Killed existing reviewer window")
		}

		if tmux.WindowExists(sessionName, "review") {
			return review.RequestReReview(sessionName, reviewFull)
		}

		return review.SpawnReviewer(sessionName, ctx)
	},
}

var prCommentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on the PR",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContext()
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
			fmt.Printf("--- %s (%s) ---\n%s\n\n", c.User, c.CreatedAt.Format("2006-01-02 15:04"), c.Body)
		}

		return nil
	},
}

func writeReviewFile(body string) (string, error) {
	f, err := os.CreateTemp("", "ttal-review-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create review file: %w", err)
	}
	if _, err := f.WriteString(body); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to write review file: %w", err)
	}
	_ = f.Close()
	return f.Name(), nil
}

func init() {
	rootCmd.AddCommand(prCmd)

	prCreateCmd.Flags().String("body", "", "PR body/description")
	prModifyCmd.Flags().String("title", "", "New PR title")
	prModifyCmd.Flags().String("body", "", "New PR body")

	prMergeCmd.Flags().Bool("keep-branch", false, "Don't delete the branch after merge")

	prReviewCmd.Flags().BoolVar(&reviewForce, "force", false, "Kill and respawn reviewer window")
	prReviewCmd.Flags().BoolVar(&reviewFull, "full", false, "Request full re-review (not delta)")

	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prModifyCmd)
	prCmd.AddCommand(prMergeCmd)
	prCmd.AddCommand(prReviewCmd)
	prCmd.AddCommand(prCommentCmd)

	prCommentCmd.AddCommand(prCommentCreateCmd)
	prCommentCmd.AddCommand(prCommentListCmd)
}
