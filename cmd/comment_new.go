package cmd

import (
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
		task, err := taskwarrior.ExportTaskBySessionID(jobID, "pending")
		if err != nil {
			task, err = taskwarrior.ExportTaskBySessionID(jobID, "completed")
		}
		if err != nil {
			return "", fmt.Errorf("no task for job ID %q: %w", jobID, err)
		}
		return task.UUID, nil
	}
	if agent := os.Getenv("TTAL_AGENT_NAME"); agent != "" {
		tasks, err := taskwarrior.ExportTasksByFilter("+ACTIVE", "+"+agent)
		if err != nil {
			return "", fmt.Errorf("taskwarrior query failed: %w", err)
		}
		if len(tasks) == 0 {
			return "", fmt.Errorf("no active task with +%s tag", agent)
		}
		if len(tasks) > 1 {
			return "", fmt.Errorf("multiple active tasks with +%s tag — expected exactly one", agent)
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
		var providerType, owner, repo string
		var prIndex int64
		if os.Getenv("TTAL_JOB_ID") != "" {
			if ctx, err := pr.ResolveContextWithoutProvider(); err == nil {
				if idx, err := pr.PRIndex(ctx); err == nil {
					providerType = string(ctx.Info.Provider)
					owner = ctx.Owner
					repo = ctx.Repo
					prIndex = idx
				}
			}
		}

		resp, err := daemon.CommentAdd(daemon.CommentAddRequest{
			Target:       taskUUID,
			Author:       author,
			Body:         body,
			ProviderType: providerType,
			Owner:        owner,
			Repo:         repo,
			PRIndex:      prIndex,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Comment added (round %d)\n", resp.Round)

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
			fmt.Println("No comments on this task.")
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

		fmt.Println(tbl)
		return nil
	},
}

var commentLgtmCmd = &cobra.Command{
	Use:   "lgtm",
	Short: "Approve the current task with +lgtm tag and audit trace",
	Long: `Add +lgtm tag to the current task and create an annotation trace.
Task is auto-resolved from TTAL_JOB_ID (worker) or TTAL_AGENT_NAME (manager).

The on-modify hook validates that only reviewers can set +lgtm.
The hook is a global taskwarrior hook (installed via ttal doctor --fix),
not worker-specific — it fires on ALL task modifications and checks
TTAL_AGENT_NAME == "reviewer".

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

		// Add +lgtm tag (on-modify hook enforces reviewer-only guard)
		if err := taskwarrior.ModifyTags(taskUUID, "+lgtm"); err != nil {
			return fmt.Errorf("failed to add +lgtm: %w", err)
		}

		// Add annotation trace
		trace := fmt.Sprintf("lgtm: %s at %s", author, time.Now().UTC().Format(time.RFC3339))
		if err := taskwarrior.AnnotateTask(taskUUID, trace); err != nil {
			log.Printf("+lgtm tag was set but annotation failed — safe to re-run ttal comment lgtm: %v", err)
			return fmt.Errorf("failed to annotate lgtm trace (+lgtm already set, safe to retry): %w", err)
		}

		fmt.Printf("LGTM approved by %s\n", author)
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
	switch os.Getenv("TTAL_AGENT_NAME") {
	case "reviewer":
		cfg, rt := loadConfigAndCoderRuntime()
		notifyCoder(sessionName, body, cfg, rt)
	case "coder":
		cfg, _ := loadConfigAndCoderRuntime()
		notifyReviewer(sessionName, body, cfg)
	case "plan-reviewer":
		notifyDesigner(sessionName, body)
	default:
		// Manager agents (kestrel, inke, etc.) notify plan-reviewer if window exists
		notifyPlanReviewer(sessionName)
	}
}

func notifyCoder(sessionName, body string, cfg *config.Config, rt runtime.Runtime) {
	coderWindow, err := tmux.FirstWindowExcept(sessionName, "review")
	if err != nil || coderWindow == "" {
		return
	}
	tmpl := cfg.Prompt("triage")
	if tmpl == "" {
		return
	}
	reviewFile, err := writeReviewFile(body)
	if err != nil {
		log.Printf("warning: failed to write review file: %v", err)
	}
	reviewRef := ""
	if reviewFile != "" {
		reviewRef = fmt.Sprintf(" Full review at %s —", reviewFile)
	}
	replacer := strings.NewReplacer("{{review-file}}", reviewRef)
	notification := config.RenderTemplate(replacer.Replace(tmpl), "", rt)
	if err := tmux.SendKeys(sessionName, coderWindow, notification); err != nil {
		log.Printf("warning: notify coder failed: %v", err)
	}
}

func notifyReviewer(sessionName, body string, cfg *config.Config) {
	if !tmux.WindowExists(sessionName, "review") {
		return
	}
	if err := review.RequestReReview(sessionName, false, body, cfg); err != nil {
		log.Printf("warning: re-review request failed: %v", err)
	}
}

func notifyDesigner(sessionName, body string) {
	designerWindow, err := tmux.FirstWindowExcept(sessionName, "plan-review")
	if err != nil || designerWindow == "" {
		return
	}
	if err := tmux.SendKeys(sessionName, designerWindow, body); err != nil {
		log.Printf("warning: notify designer failed: %v", err)
	}
}

func notifyPlanReviewer(sessionName string) {
	if !tmux.WindowExists(sessionName, "plan-review") {
		return
	}
	if err := tmux.SendKeys(sessionName, "plan-review",
		"Plan has been revised. Re-review and post findings via ttal comment add."); err != nil {
		log.Printf("warning: notify plan-review failed: %v", err)
	}
}

func init() {
	rootCmd.AddCommand(newCommentCmd)
	newCommentCmd.AddCommand(commentAddCmd)
	newCommentCmd.AddCommand(commentListCmd)
	newCommentCmd.AddCommand(commentLgtmCmd)
	newCommentCmd.AddCommand(commentGetCmd)
}
