package daemon

import (
	"fmt"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/project"
)

func handlePRCreate(req PRCreateRequest) PRResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	result, err := provider.CreatePR(req.Owner, req.Repo, req.Head, req.Base, req.Title, req.Body)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("create PR: %v", err)}
	}
	return PRResponse{OK: true, PRURL: result.HTMLURL, PRIndex: result.Index}
}

func handlePRModify(req PRModifyRequest) PRResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	result, err := provider.EditPR(req.Owner, req.Repo, req.Index, req.Title, req.Body)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("modify PR: %v", err)}
	}
	return PRResponse{OK: true, PRURL: result.HTMLURL, PRIndex: result.Index}
}

func handlePRMerge(req PRMergeRequest) PRResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	fetchedPR, err := provider.GetPR(req.Owner, req.Repo, req.Index)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("get PR: %v", err)}
	}
	if fetchedPR.Merged {
		return PRResponse{OK: false, AlreadyMerged: true, Error: fmt.Sprintf("PR #%d is already merged", req.Index)}
	}
	if !fetchedPR.Mergeable {
		reason := diagnosePRMergeFailure(provider, req.Owner, req.Repo, fetchedPR)
		return PRResponse{OK: false, Error: fmt.Sprintf("PR #%d is not mergeable:\n%s", req.Index, reason)}
	}
	if err := provider.MergePR(req.Owner, req.Repo, req.Index, req.DeleteBranch); err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("merge PR: %v", err)}
	}
	return PRResponse{OK: true}
}

func handlePRCheckMergeable(req PRCheckMergeableRequest) PRResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	fetchedPR, err := provider.GetPR(req.Owner, req.Repo, req.Index)
	if err != nil {
		return PRResponse{OK: false, Error: fmt.Sprintf("get PR: %v", err)}
	}
	if fetchedPR.Merged {
		return PRResponse{OK: false, Error: fmt.Sprintf("PR #%d is already merged", req.Index)}
	}
	if !fetchedPR.Mergeable {
		reason := diagnosePRMergeFailure(provider, req.Owner, req.Repo, fetchedPR)
		return PRResponse{OK: false, Error: fmt.Sprintf("PR #%d is not mergeable:\n%s", req.Index, reason)}
	}
	return PRResponse{OK: true, HeadSHA: fetchedPR.HeadSHA}
}

func handlePRGetPR(req PRGetPRRequest) PRGetPRResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRGetPRResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	fetchedPR, err := provider.GetPR(req.Owner, req.Repo, req.Index)
	if err != nil {
		return PRGetPRResponse{OK: false, Error: fmt.Sprintf("get PR: %v", err)}
	}
	return PRGetPRResponse{
		OK: true, HeadSHA: fetchedPR.HeadSHA,
		Merged: fetchedPR.Merged, Mergeable: fetchedPR.Mergeable,
		Title: fetchedPR.Title,
	}
}

func handlePRGetCombinedStatus(req PRGetCombinedStatusRequest) PRCIStatusResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRCIStatusResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	cs, err := provider.GetCombinedStatus(req.Owner, req.Repo, req.SHA)
	if err != nil {
		return PRCIStatusResponse{OK: false, Error: fmt.Sprintf("get combined status: %v", err)}
	}
	statuses := make([]PRCIStatus, len(cs.Statuses))
	for i, s := range cs.Statuses {
		statuses[i] = PRCIStatus{
			Context:     s.Context,
			State:       s.State,
			Description: s.Description,
			TargetURL:   s.TargetURL,
		}
	}
	return PRCIStatusResponse{OK: true, State: cs.State, Statuses: statuses}
}

func handlePRGetCIFailureDetails(req PRGetCIFailureDetailsRequest) PRCIFailureDetailsResponse {
	token := project.ResolveGitHubToken(req.ProjectAlias)
	provider, err := gitprovider.NewProviderByNameWithToken(req.ProviderType, token)
	if err != nil {
		return PRCIFailureDetailsResponse{OK: false, Error: fmt.Sprintf("create provider: %v", err)}
	}
	details, err := provider.GetCIFailureDetails(req.Owner, req.Repo, req.SHA)
	if err != nil {
		return PRCIFailureDetailsResponse{OK: false, Error: fmt.Sprintf("get CI failure details: %v", err)}
	}
	results := make([]PRCIFailureDetail, len(details))
	for i, d := range details {
		results[i] = PRCIFailureDetail{
			JobName:      d.JobName,
			WorkflowName: d.WorkflowName,
			HTMLURL:      d.HTMLURL,
			LogTail:      d.LogTail,
		}
	}
	return PRCIFailureDetailsResponse{OK: true, Details: results}
}

// diagnosePRMergeFailure queries CI status and returns a human-readable explanation.
func diagnosePRMergeFailure(
	provider gitprovider.Provider, owner, repo string, fetchedPR *gitprovider.PullRequest,
) string {
	const possibleCauses = "Possible causes: merge conflicts or branch protection rules."
	if fetchedPR.HeadSHA == "" {
		return "  Could not determine HEAD SHA to check CI status.\n  " + possibleCauses
	}
	cs, err := provider.GetCombinedStatus(owner, repo, fetchedPR.HeadSHA)
	if err != nil {
		return fmt.Sprintf("  Could not fetch CI status: %v\n  %s", err, possibleCauses)
	}
	failing, pending := countPRCheckStates(cs.Statuses)
	if pending > 0 && failing == 0 {
		return fmt.Sprintf("  CI checks still running (%d pending).\n  Try again in 30s: sleep 30 && ttal go <uuid>", pending)
	}
	return buildPRStatusLines(cs.Statuses, failing, pending)
}

// countPRCheckStates returns the number of failed and pending checks.
func countPRCheckStates(statuses []*gitprovider.CommitStatus) (failing, pending int) {
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

// buildPRStatusLines formats CI failure details as human-readable lines.
func buildPRStatusLines(statuses []*gitprovider.CommitStatus, failing, pending int) string {
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
