package pr

import (
	"fmt"
	"strconv"

	forgejoapi "codeberg.org/clawteam/ttal-cli/internal/forgejo"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Create creates a PR on Forgejo and stores the PR index in the task's pr_id UDA.
func Create(ctx *Context, title, body string) (*forgejo_sdk.PullRequest, error) {
	if ctx.Task.Branch == "" {
		return nil, fmt.Errorf("task has no branch UDA set")
	}

	c, err := forgejoapi.Client()
	if err != nil {
		return nil, err
	}

	pr, _, err := c.CreatePullRequest(ctx.Owner, ctx.Repo, forgejo_sdk.CreatePullRequestOption{
		Head:  ctx.Task.Branch,
		Base:  "main",
		Title: title,
		Body:  body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Store PR index in task UDA
	if err := taskwarrior.SetPRID(ctx.Task.UUID, strconv.FormatInt(pr.Index, 10)); err != nil {
		// Non-fatal — PR was created, just couldn't update task
		fmt.Printf("warning: PR created but failed to update task: %v\n", err)
	}

	return pr, nil
}

// Modify updates the title and/or body of an existing PR.
func Modify(ctx *Context, title, body string) (*forgejo_sdk.PullRequest, error) {
	index, err := prIndex(ctx)
	if err != nil {
		return nil, err
	}

	c, err := forgejoapi.Client()
	if err != nil {
		return nil, err
	}

	opt := forgejo_sdk.EditPullRequestOption{}
	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = body
	}

	pr, _, err := c.EditPullRequest(ctx.Owner, ctx.Repo, index, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to modify PR #%d: %w", index, err)
	}

	return pr, nil
}

// CommentCreate adds a comment to the PR.
func CommentCreate(ctx *Context, body string) (*forgejo_sdk.Comment, error) {
	index, err := prIndex(ctx)
	if err != nil {
		return nil, err
	}

	c, err := forgejoapi.Client()
	if err != nil {
		return nil, err
	}

	comment, _, err := c.CreateIssueComment(ctx.Owner, ctx.Repo, index, forgejo_sdk.CreateIssueCommentOption{
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to comment on PR #%d: %w", index, err)
	}

	return comment, nil
}

// CommentList lists comments on the PR.
func CommentList(ctx *Context) ([]*forgejo_sdk.Comment, error) {
	index, err := prIndex(ctx)
	if err != nil {
		return nil, err
	}

	c, err := forgejoapi.Client()
	if err != nil {
		return nil, err
	}

	comments, _, err := c.ListIssueComments(ctx.Owner, ctx.Repo, index, forgejo_sdk.ListIssueCommentOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list comments on PR #%d: %w", index, err)
	}

	return comments, nil
}

// Merge squash-merges the PR and deletes the branch.
// Returns an error if the PR is not mergeable (e.g. failing checks).
func Merge(ctx *Context, deleteAfterMerge bool) error {
	index, err := prIndex(ctx)
	if err != nil {
		return err
	}

	c, err := forgejoapi.Client()
	if err != nil {
		return err
	}

	// Check mergeability first
	fetchedPR, _, err := c.GetPullRequest(ctx.Owner, ctx.Repo, index)
	if err != nil {
		return fmt.Errorf("failed to get PR #%d: %w", index, err)
	}
	if fetchedPR.HasMerged {
		return fmt.Errorf("PR #%d is already merged", index)
	}
	if !fetchedPR.Mergeable {
		return fmt.Errorf("PR #%d is not mergeable (check for conflicts or failing CI)", index)
	}

	merged, _, err := c.MergePullRequest(ctx.Owner, ctx.Repo, index, forgejo_sdk.MergePullRequestOption{
		Style:                  forgejo_sdk.MergeStyleSquash,
		DeleteBranchAfterMerge: deleteAfterMerge,
	})
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", index, err)
	}
	if !merged {
		return fmt.Errorf("PR #%d merge was rejected by the server", index)
	}

	return nil
}

func prIndex(ctx *Context) (int64, error) {
	if ctx.Task.PRID == "" {
		return 0, fmt.Errorf("no PR associated with this task (create one first with: ttal pr create)")
	}
	index, err := strconv.ParseInt(ctx.Task.PRID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid pr_id %q: %w", ctx.Task.PRID, err)
	}
	return index, nil
}
