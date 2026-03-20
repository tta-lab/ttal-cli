package pr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

func Create(ctx *Context, title, body string) (*gitprovider.PullRequest, error) {
	branch, branchErr := worker.WorktreeBranch(ctx.Task.UUID, ctx.Task.Project)
	if branchErr != nil {
		branch = ctx.Task.Branch // fallback to stored UDA for backward compat
	}
	if branch == "" {
		return nil, fmt.Errorf("cannot determine branch — no active worktree for task %s", ctx.Task.UUID)
	}

	base := ctx.Info.DefaultBranch
	if base == "" {
		base = "main"
	}

	pr, err := ctx.Provider.CreatePR(ctx.Owner, ctx.Repo, branch, base, title, body)
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

// CheckMergeable verifies the PR is not already merged and is mergeable.
// Returns an error if the PR cannot be merged (conflicts, failing CI, already merged).
func CheckMergeable(ctx *Context) error {
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
		reason := diagnoseMergeFailure(ctx, fetchedPR)
		return fmt.Errorf("PR #%d is not mergeable:\n%s", index, reason)
	}

	return nil
}

func Merge(ctx *Context, deleteAfterMerge bool) error {
	// Gate: reviewer must have approved via +lgtm tag
	if !ctx.Task.HasTag("lgtm") {
		return fmt.Errorf(
			"PR not approved — reviewer must: task %s modify +lgtm",
			ctx.Task.UUID,
		)
	}

	index, err := prIndex(ctx)
	if err != nil {
		return err
	}

	if err := CheckMergeable(ctx); err != nil {
		return err
	}

	return ctx.Provider.MergePR(ctx.Owner, ctx.Repo, index, deleteAfterMerge)
}

// diagnoseMergeFailure queries CI status and returns a human-readable explanation.
func diagnoseMergeFailure(ctx *Context, pr *gitprovider.PullRequest) string {
	const possibleCauses = "Possible causes: merge conflicts or branch protection rules."
	if pr.HeadSHA == "" {
		return "  Could not determine HEAD SHA to check CI status.\n  " + possibleCauses
	}

	cs, err := ctx.Provider.GetCombinedStatus(ctx.Owner, ctx.Repo, pr.HeadSHA)
	if err != nil {
		return fmt.Sprintf("  Could not fetch CI status: %v\n  %s", err, possibleCauses)
	}

	failing, pending := countCheckStates(cs.Statuses)

	// When checks are still running and none have failed, give an actionable retry message.
	if pending > 0 && failing == 0 {
		return fmt.Sprintf("  CI checks still running (%d pending).\n  Try again in 30s: sleep 30 && ttal pr merge", pending)
	}

	return buildStatusLines(cs.Statuses, failing, pending)
}

// countCheckStates returns the number of failed and pending checks.
func countCheckStates(statuses []*gitprovider.CommitStatus) (failing, pending int) {
	for _, s := range statuses {
		switch s.State {
		case gitprovider.StateFailure, gitprovider.StateError:
			failing++
		case gitprovider.StatePending:
			pending++
		}
	}
	return
}

// buildStatusLines formats failure details and a summary line for non-pending states.
func buildStatusLines(statuses []*gitprovider.CommitStatus, failing, pending int) string {
	var lines []string
	for _, s := range statuses {
		if s.State != gitprovider.StateFailure && s.State != gitprovider.StateError {
			continue
		}
		line := fmt.Sprintf("  ✗ %s — %s", s.Context, s.Description)
		if s.TargetURL != "" {
			line += fmt.Sprintf("\n    %s", s.TargetURL)
		}
		lines = append(lines, line)
	}

	if failing > 0 {
		lines = append([]string{fmt.Sprintf("  %d CI check(s) failed:", failing)}, lines...)
	}
	if failing > 0 && pending > 0 {
		lines = append(lines, fmt.Sprintf("  ⏳ %d check(s) still pending", pending))
	}

	if failing == 0 && pending == 0 {
		if len(statuses) == 0 {
			lines = append(lines, "  No CI checks found. Likely cause: merge conflicts or branch protection rules.")
		} else {
			lines = append(lines, "  All CI checks passed. Likely cause: merge conflicts or branch protection rules.")
		}
	}

	return strings.Join(lines, "\n")
}

// PRIndex returns the PR index from the task's PRID UDA.
func PRIndex(ctx *Context) (int64, error) {
	return prIndex(ctx)
}

func prIndex(ctx *Context) (int64, error) {
	if ctx.Task.PRID == "" {
		return 0, fmt.Errorf("no PR associated with this task (create one first with: ttal pr create)")
	}
	info, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return 0, err
	}
	return info.Index, nil
}
