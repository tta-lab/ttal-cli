package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

var (
	commentVerdict string
	commentList    bool
)

var taskCommentCmd = &cobra.Command{
	Use:   "comment <uuid> [message]",
	Short: "Comment on a task or pipeline stage review",
	Long: `Post a comment on a task, routed to the appropriate backend based on pipeline config.

Backend is selected from the pipeline stage's "comments" field:
  - comments = "pr"        → post to the task's linked PR
  - comments = "flicknote" → not yet supported
  - comments = "task"      → annotate the task (default)

--verdict lgtm      : post the comment AND add the +lgtm tag (pipeline review passed)
--verdict needs_work: post the comment only (no tag — absence of +lgtm = needs work)

NOTE: +lgtm (pipeline verdict tag) is distinct from the pr_id:lgtm PR approval used
by the reviewer merge flow. They coexist and serve different purposes.

Examples:
  ttal task comment abc12345 "Looks good — LGTM"
  ttal task comment abc12345 "Needs work on error handling" --verdict needs_work
  ttal task comment abc12345 "Design approved" --verdict lgtm`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := args[0]
		if err := taskwarrior.ValidateUUID(uuid); err != nil {
			return err
		}

		if commentVerdict != "" && commentVerdict != "lgtm" && commentVerdict != "needs_work" {
			return fmt.Errorf("unknown verdict %q — use lgtm or needs_work", commentVerdict)
		}

		task, err := taskwarrior.ExportTask(uuid)
		if err != nil {
			return fmt.Errorf("cannot fetch task %s: %w", uuid, err)
		}

		// Load pipeline config and build agentRoles for stage resolution.
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		teamPath := cfg.TeamPath()

		agentRoles := make(map[string]string)
		if teamPath != "" {
			agents, _ := agentfs.Discover(teamPath)
			for _, a := range agents {
				agentRoles[a.Name] = a.Role
			}
		}

		configDir := config.DefaultConfigDir()
		pipelineCfg, _ := pipeline.Load(configDir) // best-effort; fallback to task backend

		backend := resolveCommentBackend(task, pipelineCfg, agentRoles)

		if commentList {
			return listComments(uuid, task, backend, cfg)
		}

		var message string
		if len(args) >= 2 {
			message = args[1]
		}
		if message == "" {
			return fmt.Errorf("message is required (or use --list to show comments)")
		}

		if err := postComment(uuid, task, message, backend, cfg); err != nil {
			return err
		}

		if commentVerdict == "lgtm" {
			if err := taskwarrior.ModifyTags(uuid, "+lgtm"); err != nil {
				return fmt.Errorf("comment posted but +lgtm tag failed: %w", err)
			}
			fmt.Println("  ✓ Pipeline verdict: lgtm (+lgtm tag added)")
		}

		return nil
	},
}

// resolveCommentBackend returns the comment backend ("pr", "flicknote", or "task")
// based on the task's current pipeline stage configuration.
func resolveCommentBackend(task *taskwarrior.Task, pipelineCfg *pipeline.Config, agentRoles map[string]string) string {
	if pipelineCfg == nil {
		return "task"
	}
	_, p, err := pipelineCfg.MatchPipeline(task.Tags)
	if err != nil || p == nil {
		return "task"
	}
	_, stage, err := p.CurrentStage(task.Tags, agentRoles)
	if err != nil || stage == nil {
		return "task"
	}
	if stage.Comments != "" {
		return stage.Comments
	}
	return "task"
}

// postComment routes the comment to the appropriate backend.
func postComment(uuid string, task *taskwarrior.Task, message, backend string, cfg *config.Config) error {
	switch backend {
	case "pr":
		return postPRComment(task, message)
	case "flicknote":
		return fmt.Errorf("flicknote comment backend not yet supported — use task annotations")
	default:
		if err := taskwarrior.AnnotateTask(uuid, message); err != nil {
			return err
		}
		fmt.Printf("Annotation added to task %s\n", uuid[:8])
		return nil
	}
}

// postPRComment resolves the PR context for the task and posts a comment.
func postPRComment(task *taskwarrior.Task, message string) error {
	ctx, err := pr.ResolveContextWithoutProvider()
	if err != nil {
		return fmt.Errorf("PR comment: resolve context: %w", err)
	}

	idx, err := pr.PRIndex(ctx)
	if err != nil {
		return fmt.Errorf("PR comment: resolve PR index: %w", err)
	}

	resp, err := daemon.PRCommentCreate(daemon.PRCommentCreateRequest{
		ProviderType: string(ctx.Info.Provider),
		Owner:        ctx.Owner,
		Repo:         ctx.Repo,
		Index:        idx,
		Body:         message,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Comment added to PR: %s\n", resp.PRURL)
	return nil
}

// listComments lists comments from the appropriate backend.
func listComments(uuid string, task *taskwarrior.Task, backend string, cfg *config.Config) error {
	switch backend {
	case "pr":
		ctx, err := pr.ResolveContextWithoutProvider()
		if err != nil {
			return fmt.Errorf("PR comment list: resolve context: %w", err)
		}
		idx, err := pr.PRIndex(ctx)
		if err != nil {
			return fmt.Errorf("PR comment list: resolve PR index: %w", err)
		}
		resp, err := daemon.PRCommentList(daemon.PRCommentListRequest{
			ProviderType: string(ctx.Info.Provider),
			Owner:        ctx.Owner,
			Repo:         ctx.Repo,
			Index:        idx,
		})
		if err != nil {
			return err
		}
		for _, c := range resp.Comments {
			fmt.Printf("[%s] %s: %s\n", c.CreatedAt, c.User, c.Body)
		}
		return nil
	case "flicknote":
		return fmt.Errorf("flicknote comment backend not yet supported")
	default:
		// Show task annotations.
		t, err := taskwarrior.ExportTask(uuid)
		if err != nil {
			return err
		}
		if len(t.Annotations) == 0 {
			fmt.Fprintln(os.Stderr, "No annotations on this task")
			return nil
		}
		for _, ann := range t.Annotations {
			fmt.Printf("[%s] %s\n", ann.Entry, ann.Description)
		}
		return nil
	}
}

func init() {
	taskCommentCmd.Flags().StringVar(&commentVerdict, "verdict", "", "Pipeline review verdict: lgtm or needs_work")
	taskCommentCmd.Flags().BoolVar(&commentList, "list", false, "List comments instead of posting")
}
