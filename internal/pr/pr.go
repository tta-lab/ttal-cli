package pr

import (
	"fmt"
	"strconv"

	"codeberg.org/clawteam/ttal-cli/internal/gitprovider"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

func Create(ctx *Context, title, body string) (*gitprovider.PullRequest, error) {
	if ctx.Task.Branch == "" {
		return nil, fmt.Errorf("task has no branch UDA set")
	}

	pr, err := ctx.Provider.CreatePR(ctx.Owner, ctx.Repo, ctx.Task.Branch, "main", title, body)
	if err != nil {
		return nil, err
	}

	if err := taskwarrior.SetPRID(ctx.Task.UUID, strconv.FormatInt(pr.Index, 10)); err != nil {
		fmt.Printf("warning: PR created but failed to update task: %v\n", err)
	}

	return pr, nil
}

func Modify(ctx *Context, title, body string) (*gitprovider.PullRequest, error) {
	index, err := prIndex(ctx)
	if err != nil {
		return nil, err
	}

	return ctx.Provider.EditPR(ctx.Owner, ctx.Repo, index, title, body)
}

func CommentCreate(ctx *Context, body string) (*gitprovider.Comment, error) {
	index, err := prIndex(ctx)
	if err != nil {
		return nil, err
	}

	return ctx.Provider.CreateComment(ctx.Owner, ctx.Repo, index, body)
}

func CommentList(ctx *Context) ([]*gitprovider.Comment, error) {
	index, err := prIndex(ctx)
	if err != nil {
		return nil, err
	}

	return ctx.Provider.ListComments(ctx.Owner, ctx.Repo, index)
}

func Merge(ctx *Context, deleteAfterMerge bool) error {
	index, err := prIndex(ctx)
	if err != nil {
		return err
	}

	fetchedPR, err := ctx.Provider.GetPR(ctx.Owner, ctx.Repo, index)
	if err != nil {
		return err
	}
	if fetchedPR.Merged {
		return fmt.Errorf("PR #%d is already merged", index)
	}
	if !fetchedPR.Mergeable {
		return fmt.Errorf("PR #%d is not mergeable (check for conflicts or failing CI)", index)
	}

	return ctx.Provider.MergePR(ctx.Owner, ctx.Repo, index, deleteAfterMerge)
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
