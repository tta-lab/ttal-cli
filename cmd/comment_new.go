package cmd

import (
	"fmt"
	"io"
	"os"
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
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

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
		if err != nil || len(tasks) == 0 {
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
	if role := os.Getenv("TTAL_ROLE"); role != "" {
		return role
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

		resp, err := daemon.CommentAdd(daemon.CommentAddRequest{
			Target: taskUUID,
			Author: author,
			Body:   body,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Comment added (round %d)\n", resp.Round)

		// Forge side-effect: if worker plane and task has pr_id, also post to forge
		if os.Getenv("TTAL_JOB_ID") != "" {
			task, taskErr := taskwarrior.ExportTask(taskUUID)
			if taskErr == nil && task.PRID != "" {
				prCtx, ctxErr := pr.ResolveContextWithoutProvider()
				if ctxErr == nil {
					idx, idxErr := pr.PRIndex(prCtx)
					if idxErr == nil {
						_, prErr := daemon.PRCommentCreate(daemon.PRCommentCreateRequest{
							ProviderType: string(prCtx.Info.Provider),
							Owner:        prCtx.Owner,
							Repo:         prCtx.Repo,
							Index:        idx,
							Body:         body,
						})
						if prErr != nil {
							fmt.Fprintf(os.Stderr, "warning: forge comment failed: %v\n", prErr)
						}
					}
				}
			}
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
			fmt.Println("No comments on this task.")
			return nil
		}

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()
		rows := make([][]string, 0, len(resp.Comments))
		for _, c := range resp.Comments {
			ts := c.CreatedAt
			if t, err := time.Parse(time.RFC3339, c.CreatedAt); err == nil {
				ts = t.Format("2006-01-02 15:04")
			}
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

// notifyCounterpart sends a tmux notification to the counterpart window based on TTAL_ROLE.
func notifyCounterpart(body string) {
	role := tmux.Role()
	sessionName, err := review.ResolveSessionName()
	if err != nil || sessionName == "" {
		return
	}

	cfg, rt := loadConfigAndCoderRuntime()

	switch role {
	case "reviewer":
		// Reviewer posting → notify coder window
		coderWindow, cwErr := tmux.FirstWindowExcept(sessionName, "review")
		if cwErr != nil || coderWindow == "" {
			return
		}
		reviewFile, _ := writeReviewFile(body)
		reviewRef := ""
		if reviewFile != "" {
			reviewRef = fmt.Sprintf(" Full review at %s —", reviewFile)
		}
		tmpl := cfg.Prompt("triage")
		if tmpl == "" {
			return
		}
		replacer := strings.NewReplacer("{{review-file}}", reviewRef)
		notification := config.RenderTemplate(replacer.Replace(tmpl), "", rt)
		_ = tmux.SendKeys(sessionName, coderWindow, notification)

	case "coder":
		// Coder posting → trigger re-review
		if tmux.WindowExists(sessionName, "review") {
			_ = review.RequestReReview(sessionName, false, body, cfg)
		}

	case "plan-reviewer":
		// Plan reviewer → notify designer window
		designerWindow, err := tmux.FirstWindowExcept(sessionName, "plan-review")
		if err != nil || designerWindow == "" {
			return
		}
		_ = tmux.SendKeys(sessionName, designerWindow, body)

	case "designer":
		// Designer posting → re-trigger plan review
		if tmux.WindowExists(sessionName, "plan-review") {
			_ = tmux.SendKeys(sessionName, "plan-review",
				"Plan has been revised. Re-review and post findings via ttal comment add.")
		}
	}
}

func init() {
	rootCmd.AddCommand(newCommentCmd)
	newCommentCmd.AddCommand(commentAddCmd)
	newCommentCmd.AddCommand(commentListCmd)
}
