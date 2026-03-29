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
	"github.com/tta-lab/ttal-cli/internal/review"
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
	Short: "Create a PR from the current worker branch",
	Long: `Creates a Forgejo PR using the task's branch and project path.
Stores the PR index in the task's pr_id UDA for future commands.

Examples:
  ttal pr create "feat: add user authentication"
  ttal pr create "fix: resolve timeout bug" --body "Fixes #42"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		branch, branchErr := worker.WorktreeBranch(ctx.Task.UUID, ctx.Task.Project)
		if branchErr != nil {
			branch = ""
		}
		if branch == "" {
			return fmt.Errorf(
				"cannot determine branch — run from an active worktree: task %s",
				ctx.Task.UUID,
			)
		}

		title := strings.Join(args, " ")
		body, _ := cmd.Flags().GetString("body")

		base := ctx.Info.DefaultBranch
		if base == "" {
			base = "main"
		}

		resp, err := daemon.PRCreate(daemon.PRCreateRequest{
			ProviderType: string(ctx.Info.Provider),
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Head:         branch,
			Base:         base,
			Title:        title,
			Body:         body,
			ProjectAlias: ctx.Task.Project,
		})
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d created: %s\n", resp.PRIndex, resp.PRURL)
		fmt.Printf("  %s → %s\n", branch, base)
		fmt.Println()

		// Store PRID in taskwarrior
		if ctx.Task.UUID != "" {
			if err := taskwarrior.SetPRID(ctx.Task.UUID, strconv.FormatInt(resp.PRIndex, 10)); err != nil {
				fmt.Printf("warning: PR created but failed to update task: %v\n", err)
			}
		}

		// Notify lifecycle agent
		if err := daemon.Notify(daemon.NotifyRequest{
			Message: notification.PRCreated{
				Ctx:   notification.NewContext(ctx.Task.Project, ctx.Task.HexID(), title, ""),
				Title: title,
				URL:   resp.PRURL,
			}.Render(),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: notification failed: %v\n", err)
		}

		// Auto-advance pipeline — triggers daemon to spawn PR reviewer.
		if ctx.Task.UUID != "" {
			sessionName, _ := review.ResolveSessionName()
			workDir, _ := os.Getwd()
			advResp, advErr := daemon.AdvanceClient(daemon.AdvanceRequest{
				TaskUUID:    ctx.Task.UUID,
				AgentName:   os.Getenv("TTAL_AGENT_NAME"),
				SessionName: sessionName,
				WorkDir:     workDir,
			})
			if advErr != nil {
				fmt.Fprintf(os.Stderr, "warning: auto-spawn reviewer failed: %v\n", advErr)
			} else if advResp.Status == daemon.AdvanceStatusAdvanced {
				fmt.Printf("  Spawning reviewer...\n")
			} else {
				fmt.Printf("  Pipeline: %s\n", advResp.Status)
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

		resp, err := daemon.PRModify(daemon.PRModifyRequest{
			ProviderType: string(ctx.Info.Provider),
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        index,
			Title:        title,
			Body:         body,
			ProjectAlias: ctx.Task.Project,
		})
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d updated: %s\n", resp.PRIndex, resp.PRURL)
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
