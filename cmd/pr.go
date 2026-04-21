package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/notification"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests for the current worker task",
	Long: `Create, modify, and comment on pull requests.

Context is auto-resolved from TTAL_JOB_ID (task UUID prefix).
Provider is auto-detected from git remote URL (github.com → GitHub, else → Forgejo).

All authenticated API calls are proxied through the daemon for token isolation.`,
}

var prCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a PR from the current branch",
	Long: `Creates a Forgejo PR using the current branch and project path.
Stores the PR index in the task's pr_id UDA for future commands.

Works in both worktree and non-worktree setups.

Examples:
  ttal pr create "feat: add user authentication"
  ttal pr create "fix: resolve timeout bug" --body "Fixes #42"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		// Get workDir first for branch detection
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		// Use CurrentBranch which supports both worktree and non-worktree setups
		branch := worker.CurrentBranch(ctx.Task.UUID, ctx.Task.Project, workDir)
		if branch == "" {
			return fmt.Errorf(
				"cannot determine branch — ensure you're in a git repo with an active branch: %s",
				ctx.Task.UUID,
			)
		}

		// Push branch to origin before creating PR
		fmt.Println("Pushing branch to origin...")
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
		fmt.Println("Pushed.")

		title := strings.Join(args, " ")
		body, _ := cmd.Flags().GetString("body")

		base := ctx.Info.DefaultBranch
		if base == "" {
			base = "main"
		}

		prResp, err := daemon.PRCreate(daemon.PRCreateRequest{
			ProviderType: string(ctx.Info.Provider),
			Host:         ctx.Info.Host,
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Head:         branch,
			Base:         base,
			Title:        title,
			Body:         body,
			ProjectAlias: ctx.Alias,
		})
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d created: %s\n", prResp.PRIndex, prResp.PRURL)
		fmt.Printf("  %s → %s\n", branch, base)
		fmt.Println()

		// Store PRID in taskwarrior
		if ctx.Task.UUID != "" {
			if err := taskwarrior.SetPRID(ctx.Task.UUID, strconv.FormatInt(prResp.PRIndex, 10)); err != nil {
				fmt.Printf("warning: PR created but failed to update task: %v\n", err)
			}
		}

		// Notify lifecycle agent
		if err := daemon.Notify(daemon.NotifyRequest{
			Message: notification.PRCreated{
				Ctx:   notification.NewContext(ctx.Task.Project, ctx.Task.HexID(), title, ""),
				Title: title,
				URL:   prResp.PRURL,
			}.Render(),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: notification failed: %v\n", err)
		}

		// Notify the task owner so they can review the PR against their plan.
		// Owner decides when to advance via `ttal go <hex>` (which spawns pr-review-lead).
		if ctx.Task.UUID != "" {
			// ctx.Task.Owner is the taskwarrior Owner UDA (distinct from ctx.Owner, the repo owner).
			owner := ctx.Task.Owner
			if owner == "" {
				fmt.Fprintf(os.Stderr, "warning: task %s has no owner — skipping owner-review notification\n", ctx.Task.HexID())
			} else {
				assignee := os.Getenv("TTAL_AGENT_NAME")
				if assignee == "" {
					assignee = "coder"
				}
				worktree, _ := worker.WorktreePath(ctx.Task.UUID, ctx.Task.Project)
				msg := pr.BuildOwnerReviewMessage(prResp.PRIndex, prResp.PRURL, title, worktree, ctx.Task.HexID(), assignee)
				if err := daemon.Send(daemon.SendRequest{From: "system", To: owner, Message: msg}); err != nil {
					fmt.Fprintf(os.Stderr, "warning: owner-review notification failed: %v\n", err)
				} else {
					fmt.Printf("  Notified %s for review.\n", owner)
				}
			}
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
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")

		if title == "" && body == "" {
			return fmt.Errorf("specify --title, --body, or both\n\n  Example: ttal pr modify --title \"new title\" --body \"updated description\"") //nolint:lll
		}

		index, err := pr.PRIndex(ctx)
		if err != nil {
			return err
		}

		prResp, err := daemon.PRModify(daemon.PRModifyRequest{
			ProviderType: string(ctx.Info.Provider),
			Host:         ctx.Info.Host,
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        index,
			Title:        title,
			Body:         body,
			ProjectAlias: ctx.Alias,
		})
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d updated: %s\n", prResp.PRIndex, prResp.PRURL)
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

// resolveCoderRuntime returns the coder's runtime from TTAL_RUNTIME env var,
// falling back to ClaudeCode if unset or invalid.
func resolveCoderRuntime() runtime.Runtime {
	if env := os.Getenv("TTAL_RUNTIME"); env != "" {
		if r, err := runtime.Parse(env); err == nil {
			return r
		}
	}
	return runtime.ClaudeCode
}

// loadConfigAndCoderRuntime loads config and resolves the coder runtime.
// Falls back to empty config + ClaudeCode on error.
func loadConfigAndCoderRuntime() (*config.Config, runtime.Runtime) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		cfg = &config.Config{}
	}
	return cfg, resolveCoderRuntime()
}

func init() {
	rootCmd.AddCommand(prCmd)

	prCreateCmd.Flags().String("body", "", "PR body/description")
	prModifyCmd.Flags().String("title", "", "New PR title")
	prModifyCmd.Flags().String("body", "", "New PR body")

	prCICmd.Flags().BoolVar(&prCIShowLog, "log", false, "Include failure details and log tails")

	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prModifyCmd)
	prCmd.AddCommand(prCICmd)
}
