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

var (
	daemonPRCreateFn   = daemon.PRCreate                  // inject point for tests
	daemonPRModifyFn   = daemon.PRModify                  // inject point for tests
	prResolveContextFn = pr.ResolveContextWithoutProvider // inject point for tests
	currentBranchFn    = worker.CurrentBranch             // inject point for tests
	gitPushFn          = daemon.GitPush                   // inject point for tests
	setPRIDFn          = taskwarrior.SetPRID              // inject point for tests
	daemonNotifyFn     = daemon.Notify                    // inject point for tests
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests for the current worker task",
	Long: `Create, modify, and comment on pull requests.

Context is auto-resolved from the current worktree path (hex ID in directory name).
Provider is auto-detected from git remote URL (github.com → GitHub, else → Forgejo).

All authenticated API calls are proxied through the daemon for token isolation.`,
}

var prCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a PR from the current branch",
	Long: `Creates a PR using the current branch and project path.
Stores the PR index in the task's pr_id UDA for future commands.

Works in both worktree and non-worktree setups.

Body is read from stdin (heredoc). Title is the positional argument.

Examples:
  ttal pr create "feat: add user authentication"
  echo "Fixes #42" | ttal pr create "fix: resolve timeout bug"
  cat <<'BODY' | ttal pr create "feat: major refactor"
  Comprehensive description spanning multiple lines.
  BODY`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := prResolveContextFn()
		if err != nil {
			return err
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		// Use CurrentBranch which supports both worktree and non-worktree setups
		branch := currentBranchFn(ctx.Task.UUID, ctx.Task.Project, workDir)
		if branch == "" {
			return fmt.Errorf(
				"cannot determine branch — ensure you're in a git repo with an active branch: %s",
				ctx.Task.UUID,
			)
		}

		// Push branch to origin before creating PR
		fmt.Println("Pushing branch to origin...")
		resp, err := gitPushFn(daemon.GitPushRequest{
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
		body, err := readStdinIfPiped()
		if err != nil {
			return fmt.Errorf("read PR body from stdin: %w", err)
		}

		base := ctx.Info.DefaultBranch
		if base == "" {
			base = "main"
		}

		prResp, err := daemonPRCreateFn(daemon.PRCreateRequest{
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
			if err := setPRIDFn(ctx.Task.UUID, strconv.FormatInt(prResp.PRIndex, 10)); err != nil {
				fmt.Printf("warning: PR created but failed to update task: %v\n", err)
			}
		}

		// Notify lifecycle agent
		if err := daemonNotifyFn(daemon.NotifyRequest{
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
				worktree, err := worker.WorktreePath(ctx.Task.UUID, ctx.Task.Project)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not resolve worktree path for task %s: %v\n", ctx.Task.HexID(), err)
				}
				msg := pr.BuildOwnerReviewMessage(prResp.PRIndex, prResp.PRURL, title, worktree, ctx.Task.HexID(), assignee)
				if err := daemonSendFn(daemon.SendRequest{From: "system", To: owner, Message: msg}); err != nil {
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

Title is set with --title. Body is read from stdin (heredoc). At least one of --title or stdin must be provided.

Examples:
  echo "Updated body content" | ttal pr modify --title "new title"
  cat <<'BODY' | ttal pr modify
  New PR body content.
  BODY`,
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		body, err := readStdinIfPiped()
		if err != nil {
			return fmt.Errorf("read PR body from stdin: %w", err)
		}

		if title == "" && body == "" {
			return fmt.Errorf("nothing to update — provide --title and/or pipe body via stdin\n\n" +
				"  Title only:  ttal pr modify --title \"new title\"\n" +
				"  Body only:   echo \"new body\" | ttal pr modify\n" +
				"  Both:        echo \"new body\" | ttal pr modify --title \"new title\"\n" +
				"  Heredoc:     cat <<'EOF' | ttal pr modify --title \"new title\"\n" +
				"                 ## Updated\n" +
				"                 ...\n" +
				"               EOF")
		}

		ctx, err := prResolveContextFn()
		if err != nil {
			return err
		}

		// --pr-id overrides PR lookup (for non-worktree use)
		if prIDFlag := cmd.Flag("pr-id").Value.String(); prIDFlag != "" {
			ctx.Task.PRID = prIDFlag
		}

		index, err := pr.PRIndex(ctx)
		if err != nil {
			return err
		}

		prResp, err := daemonPRModifyFn(daemon.PRModifyRequest{
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

	prModifyCmd.Flags().String("title", "", "New PR title")
	prModifyCmd.Flags().String("pr-id", "", "PR number override (for non-worktree use)")

	prCICmd.Flags().BoolVar(&prCIShowLog, "log", false, "Include failure details and log tails")

	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prModifyCmd)
	prCmd.AddCommand(prCICmd)
}
