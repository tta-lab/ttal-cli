package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
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

		// Update ctx.Task.PRID for SpawnReviewer
		ctx.Task.PRID = strconv.FormatInt(resp.PRIndex, 10)

		// Notify lifecycle agent
		worker.NotifyTelegram(fmt.Sprintf("📋 PR created: %s\n%s", title, resp.PRURL))

		// Auto-spawn reviewer
		sessionName, sessionErr := review.ResolveSessionName()
		if sessionErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to detect tmux session: %v\n", sessionErr)
		} else if sessionName != "" {
			cfg, _ := loadConfigAndCoderRuntime()
			if tmux.WindowExists(sessionName, "review") {
				fmt.Println("  Reviewer already running, sending review request...")
				if err := review.RequestReReview(sessionName, false, "", cfg); err != nil {
					fmt.Fprintf(os.Stderr, "warning: re-review request failed: %v\n", err)
				}
			} else {
				fmt.Println("  Spawning reviewer...")
				reviewerName := resolvePRReviewerName(ctx.Task.Tags)
				if err := review.SpawnReviewer(sessionName, ctx, reviewerName, cfg); err != nil {
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
		})
		if err != nil {
			return err
		}

		fmt.Printf("PR #%d updated: %s\n", resp.PRIndex, resp.PRURL)
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
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		// Check LGTM gate via +lgtm tag
		if !ctx.Task.HasTag("lgtm") {
			return fmt.Errorf("PR not approved — reviewer must run: ttal comment lgtm")
		}

		keepBranch, _ := cmd.Flags().GetBool("keep-branch")

		// Check for uncommitted changes before merging
		statusOut, statusErr := exec.Command("git", "status", "--porcelain").Output()
		if statusErr != nil {
			return fmt.Errorf("failed to check worktree status: %w", statusErr)
		}
		if strings.TrimSpace(string(statusOut)) != "" {
			return fmt.Errorf("worktree has uncommitted changes — commit or stash before merging")
		}

		prIndex, err := pr.PRIndex(ctx)
		if err != nil {
			return err
		}

		prURL := pr.BuildPRURL(ctx)

		// Check mergeability via daemon
		if _, err := daemon.PRCheckMergeable(daemon.PRCheckMergeableRequest{
			ProviderType: string(ctx.Info.Provider),
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        prIndex,
		}); err != nil {
			worker.NotifyTelegram(fmt.Sprintf("⏳ PR not mergeable: %s\n%s\n%v", ctx.Task.Description, prURL, err))
			return err
		}

		// Resolve merge mode from config (team > global > "auto")
		mergeMode := config.MergeModeAuto
		cfg, cfgErr := config.Load()
		if cfgErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load config, defaulting to auto merge: %v\n", cfgErr)
		} else {
			mergeMode = cfg.GetMergeMode()
		}

		if mergeMode == config.MergeModeManual {
			worker.NotifyTelegram(fmt.Sprintf("🔔 PR ready to merge: %s\n%s", ctx.Task.Description, prURL))
			fmt.Printf("PR #%s is mergeable (manual mode — notification sent)\n", ctx.Task.PRID)
			return nil
		}

		// Merge via daemon
		if _, err = daemon.PRMerge(daemon.PRMergeRequest{
			ProviderType: string(ctx.Info.Provider),
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        prIndex,
			DeleteBranch: !keepBranch,
		}); err != nil {
			return err
		}

		fmt.Printf("PR #%s merged (squash)\n", ctx.Task.PRID)
		if !keepBranch {
			fmt.Println("  Branch deleted")
		}

		// Fire-and-forget: request daemon cleanup (session + worktree + task done)
		if err := worker.RequestCleanup(ctx.Task.SessionName(), ctx.Task.UUID); err != nil {
			fmt.Fprintf(os.Stderr,
				"warning: cleanup request failed: %v\n  run: ttal worker close %s\n",
				err, ctx.Task.SessionName())
		} else {
			fmt.Println("  Cleanup requested (daemon will close session + worktree)")
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
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return err
		}

		sessionName, err := review.ResolveSessionName()
		if err != nil {
			return fmt.Errorf("failed to detect tmux session: %w", err)
		}
		if sessionName == "" {
			return fmt.Errorf("must be run inside a tmux session (this command is for worker sessions)")
		}

		if reviewForce && tmux.WindowExists(sessionName, "review") {
			if err := tmux.KillWindow(sessionName, "review"); err != nil {
				return fmt.Errorf("failed to kill reviewer window: %w", err)
			}
			fmt.Println("Killed existing reviewer window")
		}

		cfg, _ := loadConfigAndCoderRuntime()
		if tmux.WindowExists(sessionName, "review") {
			return review.RequestReReview(sessionName, reviewFull, "", cfg)
		}

		reviewerName := resolvePRReviewerName(ctx.Task.Tags)
		return review.SpawnReviewer(sessionName, ctx, reviewerName, cfg)
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

// resolvePRReviewerName resolves the PR reviewer agent name from pipeline config.
// Falls back to "pr-review-lead" if no pipeline matches or no reviewer is configured.
func resolvePRReviewerName(taskTags []string) string {
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load pipelines.toml — falling back to pr-review-lead: %v\n", err)
		return "pr-review-lead"
	}
	if name := pipelineCfg.ReviewerForStage(taskTags, "worker"); name != "" {
		return name
	}
	return "pr-review-lead"
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

	prMergeCmd.Flags().Bool("keep-branch", false, "Don't delete the branch after merge")

	prReviewCmd.Flags().BoolVar(&reviewForce, "force", false, "Kill and respawn reviewer window")
	prReviewCmd.Flags().BoolVar(&reviewFull, "full", false, "Request full re-review (not delta)")

	prCICmd.Flags().BoolVar(&prCIShowLog, "log", false, "Include failure details and log tails")

	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prModifyCmd)
	prCmd.AddCommand(prMergeCmd)
	prCmd.AddCommand(prReviewCmd)
	prCmd.AddCommand(prCICmd)
}
