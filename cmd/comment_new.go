package cmd

import (
	"fmt"
	"io"
	"log"
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
	"github.com/tta-lab/ttal-cli/internal/runtime"
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
			if taskErr != nil {
				fmt.Fprintf(os.Stderr, "warning: forge comment skipped: export task: %v\n", taskErr)
			} else if task.PRID != "" {
				prCtx, ctxErr := pr.ResolveContextWithoutProvider()
				if ctxErr != nil {
					fmt.Fprintf(os.Stderr, "warning: forge comment skipped: resolve context: %v\n", ctxErr)
				} else {
					idx, idxErr := pr.PRIndex(prCtx)
					if idxErr != nil {
						fmt.Fprintf(os.Stderr, "warning: forge comment skipped: PR index: %v\n", idxErr)
					} else {
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

// notifyCounterpart sends a tmux notification to the counterpart window based on TTAL_AGENT_NAME.
func notifyCounterpart(body string) {
	sessionName, err := review.ResolveSessionName()
	if err != nil || sessionName == "" {
		return
	}
	cfg, rt := loadConfigAndCoderRuntime()

	switch os.Getenv("TTAL_AGENT_NAME") {
	case "reviewer":
		notifyReviewer(sessionName, body, cfg, rt)
	case "coder":
		notifyCoder(sessionName, body, cfg)
	case "plan-reviewer":
		notifyPlanReviewer(sessionName, body)
	case "designer":
		notifyDesigner(sessionName)
	}
}

func notifyReviewer(sessionName, body string, cfg *config.Config, rt runtime.Runtime) {
	coderWindow, err := tmux.FirstWindowExcept(sessionName, "review")
	if err != nil || coderWindow == "" {
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
	tmpl := cfg.Prompt("triage")
	if tmpl == "" {
		return
	}
	replacer := strings.NewReplacer("{{review-file}}", reviewRef)
	notification := config.RenderTemplate(replacer.Replace(tmpl), "", rt)
	if err := tmux.SendKeys(sessionName, coderWindow, notification); err != nil {
		log.Printf("warning: notify coder failed: %v", err)
	}
}

func notifyCoder(sessionName, body string, cfg *config.Config) {
	if !tmux.WindowExists(sessionName, "review") {
		return
	}
	if err := review.RequestReReview(sessionName, false, body, cfg); err != nil {
		log.Printf("warning: re-review request failed: %v", err)
	}
}

func notifyPlanReviewer(sessionName, body string) {
	designerWindow, err := tmux.FirstWindowExcept(sessionName, "plan-review")
	if err != nil || designerWindow == "" {
		return
	}
	if err := tmux.SendKeys(sessionName, designerWindow, body); err != nil {
		log.Printf("warning: notify designer failed: %v", err)
	}
}

func notifyDesigner(sessionName string) {
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
}
