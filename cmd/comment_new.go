package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/planreview"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// formatCommentTime parses an RFC3339 timestamp and returns a short display format.
func formatCommentTime(raw string) string {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return raw
}

// resolveCurrentTask returns the task UUID for the current context.
// Worker plane: TTAL_JOB_ID → session lookup.
// Manager plane: TTAL_AGENT_NAME → active task with +<agent> tag.
func resolveCurrentTask() (string, error) {
	if jobID := os.Getenv("TTAL_JOB_ID"); jobID != "" {
		task, pendingErr := taskwarrior.ExportTaskByHexID(jobID, "pending")
		if pendingErr != nil {
			var completedErr error
			task, completedErr = taskwarrior.ExportTaskByHexID(jobID, "completed")
			if completedErr != nil {
				return "", fmt.Errorf("no task for job ID %q (pending: %v; completed: %w)", jobID, pendingErr, completedErr)
			}
		}
		return task.UUID, nil
	}
	if agent := os.Getenv("TTAL_AGENT_NAME"); agent != "" {
		cfg, err := pipeline.Load(config.DefaultConfigDir())
		if err != nil {
			return "", fmt.Errorf("load pipeline config: %w", err)
		}
		tasks, err := pipeline.ActiveTasksByOwner(cfg, agent)
		if err != nil {
			return "", fmt.Errorf("task lookup failed: %w", err)
		}
		if len(tasks) == 0 {
			return "", fmt.Errorf("no active task owned by %s", agent)
		}
		if len(tasks) > 1 {
			return "", fmt.Errorf("multiple active tasks owned by %s — expected exactly one", agent)
		}
		return tasks[0].UUID, nil
	}
	return "", fmt.Errorf("no TTAL_JOB_ID or TTAL_AGENT_NAME set — cannot resolve task")
}

// resolveAuthor returns the author name from env vars.
func resolveAuthor() string {
	if agent := os.Getenv("TTAL_AGENT_NAME"); agent != "" {
		return agent
	}
	return "unknown"
}

var newCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage task comments",
}

var commentAddCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Add a comment to the current task",
	Long: `Add a comment. Task is auto-resolved from TTAL_JOB_ID (worker) or
TTAL_AGENT_NAME (manager). No explicit UUID needed.

Examples:
  ttal comment add "looks good"
  echo "multiline" | ttal comment add
  cat <<'HEREDOC' | ttal comment add
  ## Review Round 1
  Findings here.
  HEREDOC`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskUUID, err := resolveCurrentTask()
		if err != nil {
			return err
		}

		body := strings.Join(args, " ")
		if body == "" {
			bodyBytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			body = strings.TrimSpace(string(bodyBytes))
		}
		if body == "" {
			return fmt.Errorf("comment body is required (pass as argument or pipe via stdin)")
		}

		author := resolveAuthor()

		// Attempt to populate PR context for mirroring (worker plane only).
		var providerType, owner, repo, projectAlias, host string
		var prIndex int64
		if os.Getenv("TTAL_JOB_ID") != "" {
			if ctx, err := pr.ResolveContextWithoutProvider(); err != nil {
				log.Printf("debug: PR context not resolved — no mirror: %v", err)
			} else if idx, err := pr.PRIndex(ctx); err != nil {
				if !errors.Is(err, pr.ErrNoPR) {
					log.Printf("debug: PR index not resolved — no mirror: %v", err)
				}
			} else {
				providerType = string(ctx.Info.Provider)
				owner = ctx.Owner
				repo = ctx.Repo
				prIndex = idx
				projectAlias = ctx.Task.Project
				host = ctx.Info.Host
			}
		}

		resp, err := daemon.CommentAdd(daemon.CommentAddRequest{
			Target:       taskUUID,
			Author:       author,
			Body:         body,
			ProviderType: providerType,
			Host:         host,
			Owner:        owner,
			Repo:         repo,
			PRIndex:      prIndex,
			ProjectAlias: projectAlias,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Comment added (round %d)\n", resp.Round)

		// Show lgtm hint only to reviewers — they're the ones who should run it.
		if isReviewer(author) {
			fmt.Println("When ready to approve: ttal comment lgtm")
		}

		// Notify counterpart window
		notifyCounterpart(body)

		return nil
	},
}

var commentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on the current task",
	RunE: func(cmd *cobra.Command, args []string) error {
		taskUUID, err := resolveCurrentTask()
		if err != nil {
			return err
		}

		resp, err := daemon.CommentList(daemon.CommentListRequest{Target: taskUUID})
		if err != nil {
			return err
		}

		if len(resp.Comments) == 0 {
			fmt.Println("No comments yet — sit tight, notifications will come when there's" +
				" something to read. No need to keep checking.")
			return nil
		}

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()
		rows := make([][]string, 0, len(resp.Comments))
		for _, c := range resp.Comments {
			ts := formatCommentTime(c.CreatedAt)
			body := c.Body
			if len(body) > 80 {
				body = body[:77] + "..."
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", c.Round),
				c.Author,
				body,
				ts,
			})
		}

		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				if col < 2 {
					return dimStyle
				}
				return cellStyle
			}).
			Headers("ROUND", "AUTHOR", "BODY", "TIME").
			Rows(rows...)

		lipgloss.Println(tbl)
		return nil
	},
}

var commentLgtmCmd = &cobra.Command{
	Use:   "lgtm",
	Short: "Approve the current task with a stage-specific lgtm tag and audit trace",
	Long: `Add +<stagename>_lgtm tag to the current task and create an annotation trace.
Task is auto-resolved from TTAL_JOB_ID (worker) or TTAL_AGENT_NAME (manager).

The on-modify hook validates that only reviewers can set _lgtm tags.
The hook is a global taskwarrior hook (installed via ttal doctor --fix),
not worker-specific — it fires on ALL task modifications and checks
TTAL_AGENT_NAME matches a pipeline reviewer configured in pipelines.toml.

Examples:
  ttal comment lgtm`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskUUID, err := resolveCurrentTask()
		if err != nil {
			return err
		}

		author := os.Getenv("TTAL_AGENT_NAME")
		if author == "" {
			return fmt.Errorf("TTAL_AGENT_NAME not set — cannot record lgtm attribution")
		}

		// Resolve the active stage to build the stage-specific lgtm tag.
		task, err := taskwarrior.ExportTask(taskUUID)
		if err != nil {
			return fmt.Errorf("export task: %w", err)
		}
		pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
		if err != nil {
			return fmt.Errorf("load pipeline config: %w", err)
		}
		_, p, err := pipelineCfg.MatchPipeline(task.Tags)
		if err != nil {
			return fmt.Errorf("match pipeline: %w", err)
		}
		if p == nil {
			return fmt.Errorf("no pipeline matches this task — cannot determine stage for lgtm")
		}
		idx, stage, err := p.CurrentStage(task.Tags)
		if err != nil {
			return fmt.Errorf("determine current stage: %w", err)
		}
		if idx == -1 || stage == nil {
			return fmt.Errorf("no active stage found — task may not have started")
		}

		lgtmTag := stage.StageLGTMTag()

		// Add stage-specific lgtm tag (on-modify hook enforces reviewer-only guard)
		if err := taskwarrior.ModifyTags(taskUUID, "+"+lgtmTag); err != nil {
			return fmt.Errorf("failed to add +%s: %w", lgtmTag, err)
		}

		// Add annotation trace
		trace := fmt.Sprintf("lgtm: %s stage:%s at %s", author, stage.Name, time.Now().UTC().Format(time.RFC3339))
		if err := taskwarrior.AnnotateTask(taskUUID, trace); err != nil {
			log.Printf("+%s tag was set but annotation failed — safe to re-run ttal comment lgtm: %v", lgtmTag, err)
			return fmt.Errorf("failed to annotate lgtm trace (+%s already set, safe to retry): %w", lgtmTag, err)
		}

		fmt.Printf("LGTM approved by %s (stage: %s)\n", author, stage.Name)

		notifyLgtm(author)

		// Auto-close the reviewer window — job is done.
		// Delegates to the daemon so the kill happens out-of-process
		// (avoids SIGHUP to the calling CLI).
		if session, err := tmux.CurrentSession(); err == nil && session != "" {
			if window, err := tmux.CurrentWindow(); err == nil && window != "" {
				if err := daemon.CloseWindow(daemon.CloseWindowRequest{
					Session: session,
					Window:  window,
				}); err != nil {
					log.Printf("debug: auto-close reviewer window: %v", err)
				}
			}
		}
		return nil
	},
}

var commentGetCmd = &cobra.Command{
	Use:   "get <round>",
	Short: "Get comments for a specific review round",
	Long: `Retrieve the full comment body for a specific review round.
Task is auto-resolved from TTAL_JOB_ID (worker) or TTAL_AGENT_NAME (manager).

Examples:
  ttal comment get 1
  ttal comment get 2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		round, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("round must be a number: %w", err)
		}
		if round < 1 {
			return fmt.Errorf("round must be >= 1")
		}

		taskUUID, err := resolveCurrentTask()
		if err != nil {
			return err
		}

		resp, err := daemon.CommentGet(daemon.CommentGetRequest{
			Target: taskUUID,
			Round:  round,
		})
		if err != nil {
			return err
		}

		if len(resp.Comments) == 0 {
			fmt.Printf("No comments for round %d.\n", round)
			return nil
		}

		for _, c := range resp.Comments {
			fmt.Printf("— %s (%s):\n\n%s\n\n", c.Author, formatCommentTime(c.CreatedAt), c.Body)
		}
		return nil
	},
}

// isReviewer checks whether the given agent name is a reviewer in any pipeline.
func isReviewer(agentName string) bool {
	if agentName == "" || agentName == "unknown" {
		return false
	}
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		return false
	}
	return pipelineCfg.ReviewerNotifyTarget(agentName) != pipeline.NotifyTargetNone
}

// notifyCounterpart sends a tmux notification to the counterpart window based on TTAL_AGENT_NAME.
func notifyCounterpart(body string) {
	sessionName, err := review.ResolveSessionName()
	if err != nil {
		log.Printf("debug: notifyCounterpart: resolve session: %v", err)
		return
	}
	if sessionName == "" {
		return
	}

	agentName := os.Getenv("TTAL_AGENT_NAME")
	taskTags := resolveTaskTags()

	cfg, rt := loadConfigAndCoderRuntime()

	// Check if this agent is a worker — always notify the reviewer window.
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err == nil && pipelineCfg.IsWorkerAgent(agentName) {
		reviewerWindow := resolveReviewerWindow(taskTags, "coder", "pr-review-lead")
		notifyReviewer(sessionName, body, cfg, reviewerWindow)
		return
	}

	// Check if this agent is a reviewer — route based on what stage type they review.
	if err != nil {
		log.Printf("warn: notifyCounterpart: load pipelines: %v", err)
		// Pipeline unavailable — fall back to plan-review-lead directly; don't re-invoke
		// resolveReviewerWindow which would attempt another failing pipeline.Load.
		notifyPlanReviewer(sessionName, body, cfg, "plan-review-lead")
		return
	}

	switch pipelineCfg.ReviewerNotifyTarget(agentName) {
	case pipeline.NotifyTargetCoder:
		notifyCoder(sessionName, body, cfg, rt, taskTags)
	case pipeline.NotifyTargetDesigner:
		notifyDesigner(sessionName, body, cfg, rt)
	default:
		// Manager agents notify plan-reviewer if window exists.
		reviewerWindow := resolveReviewerWindow(taskTags, "designer", "plan-review-lead")
		notifyPlanReviewer(sessionName, body, cfg, reviewerWindow)
	}
}

// renderTriageNotification writes the review body to a temp file, substitutes
// {{review-file}} in tmpl, and renders the result. Returns (notification, ok) —
// ok=false means writeReviewFile failed and the caller should fall back to raw body.
func renderTriageNotification(body, tmpl string, rt runtime.Runtime) (string, bool) {
	reviewFile, err := writeReviewFile(body)
	if err != nil {
		log.Printf("warning: failed to write review file, falling back to raw body: %v", err)
		return "", false
	}
	reviewRef := fmt.Sprintf(" Full review at %s —", reviewFile)
	replacer := strings.NewReplacer("{{review-file}}", reviewRef)
	return config.RenderTemplate(replacer.Replace(tmpl), "", rt), true
}

func notifyCoder(sessionName, body string, cfg *config.Config, rt runtime.Runtime, taskTags []string) {
	windowName := workerWindowName(taskTags)
	if !tmux.WindowExists(sessionName, windowName) {
		return
	}
	tmpl := cfg.Prompt("triage")
	if tmpl == "" {
		return
	}
	notification, ok := renderTriageNotification(body, tmpl, rt)
	if !ok {
		return
	}
	if err := tmux.SendKeys(sessionName, windowName, notification); err != nil {
		log.Printf("warning: notify coder failed: %v", err)
	}
}

func notifyReviewer(sessionName, body string, cfg *config.Config, reviewerWindow string) {
	if !tmux.WindowExists(sessionName, reviewerWindow) {
		return
	}
	if err := review.RequestReReview(sessionName, reviewerWindow, false, body, cfg); err != nil {
		log.Printf("warning: re-review request failed: %v", err)
	}
}

func notifyDesigner(sessionName, body string, cfg *config.Config, rt runtime.Runtime) {
	// Designer/planner is always the first window — it is created at session spawn time
	// (internal/daemon/advance.go via tmux.NewSession) before any reviewer window is
	// added later (tmux.NewWindow appends). FirstWindow is safe without exclusion.
	designerWindow, err := tmux.FirstWindow(sessionName)
	if err != nil || designerWindow == "" {
		if err != nil {
			log.Printf("warning: could not find designer window in %s: %v", sessionName, err)
		}
		return
	}
	tmpl := cfg.Prompt("plan_triage")
	if tmpl == "" {
		log.Printf("warning: plan_triage prompt not configured — skipping notification")
		return
	}
	notification, ok := renderTriageNotification(body, tmpl, rt)
	if !ok {
		return
	}
	if err := tmux.SendKeys(sessionName, designerWindow, notification); err != nil {
		log.Printf("warning: notify designer failed: %v", err)
	}
}

func notifyPlanReviewer(sessionName, body string, cfg *config.Config, reviewerWindow string) {
	if !tmux.WindowExists(sessionName, reviewerWindow) {
		return
	}
	if err := planreview.RequestReReview(sessionName, reviewerWindow, body, cfg); err != nil {
		log.Printf("warning: notify plan-review failed: %v", err)
	}
}

// notifyLgtm sends a simple LGTM approval message to the designer window.
// Unlike notifyCounterpart, this skips the plan_triage template to avoid
// duplicating the triage notification already sent by ttal comment add.
func notifyLgtm(reviewer string) {
	sessionName, err := review.ResolveSessionName()
	if err != nil {
		log.Printf("warning: notifyLgtm: resolve session: %v", err)
		return
	}
	if sessionName == "" {
		log.Printf("debug: notifyLgtm: not inside tmux — skipping designer notification")
		return
	}

	designerWindow, err := tmux.FirstWindow(sessionName)
	if err != nil {
		log.Printf("warning: notifyLgtm: could not find designer window in %s: %v", sessionName, err)
		return
	}
	if designerWindow == "" {
		log.Printf("warning: notifyLgtm: session %s has no windows — designer notification skipped", sessionName)
		return
	}

	msg := fmt.Sprintf(
		"VERDICT: LGTM given by %s. When you finish triaging the last review comment, advance with: ttal go",
		reviewer,
	)
	if err := tmux.SendKeys(sessionName, designerWindow, msg); err != nil {
		log.Printf("warning: notifyLgtm: send failed: %v", err)
	}
}

func init() {
	rootCmd.AddCommand(newCommentCmd)
	newCommentCmd.AddCommand(commentAddCmd)
	newCommentCmd.AddCommand(commentListCmd)
	newCommentCmd.AddCommand(commentLgtmCmd)
	newCommentCmd.AddCommand(commentGetCmd)
}
