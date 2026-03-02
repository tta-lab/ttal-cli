package gitprovider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

type ForgejoProvider struct {
	client  *forgejo_sdk.Client
	baseURL string
	token   string
}

func NewForgejoProvider() (Provider, error) {
	token := os.Getenv("FORGEJO_TOKEN")
	if token == "" {
		token = os.Getenv("FORGEJO_ACCESS_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("FORGEJO_TOKEN or FORGEJO_ACCESS_TOKEN environment variable is required")
	}

	url := os.Getenv("FORGEJO_URL")
	if url == "" {
		return nil, fmt.Errorf("FORGEJO_URL environment variable is required")
	}

	client, err := forgejo_sdk.NewClient(url, forgejo_sdk.SetToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to create Forgejo client: %w", err)
	}

	return &ForgejoProvider{client: client, baseURL: url, token: token}, nil
}

func (p *ForgejoProvider) Name() string { return "forgejo" }

func (p *ForgejoProvider) CreatePR(owner, repo, head, base, title, body string) (*PullRequest, error) {
	pr, _, err := p.client.CreatePullRequest(owner, repo, forgejo_sdk.CreatePullRequestOption{
		Head:  head,
		Base:  base,
		Title: title,
		Body:  body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	return toPullRequest(pr), nil
}

func (p *ForgejoProvider) EditPR(owner, repo string, index int64, title, body string) (*PullRequest, error) {
	opt := forgejo_sdk.EditPullRequestOption{}
	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = body
	}

	pr, _, err := p.client.EditPullRequest(owner, repo, index, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to edit PR #%d: %w", index, err)
	}

	return toPullRequest(pr), nil
}

func (p *ForgejoProvider) GetPR(owner, repo string, index int64) (*PullRequest, error) {
	pr, _, err := p.client.GetPullRequest(owner, repo, index)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %w", index, err)
	}

	return toPullRequest(pr), nil
}

func (p *ForgejoProvider) MergePR(owner, repo string, index int64, deleteBranch bool) error {
	merged, _, err := p.client.MergePullRequest(owner, repo, index, forgejo_sdk.MergePullRequestOption{
		Style:                  forgejo_sdk.MergeStyleSquash,
		DeleteBranchAfterMerge: deleteBranch,
	})
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", index, err)
	}
	if !merged {
		return fmt.Errorf("PR #%d merge was rejected by the server", index)
	}

	return nil
}

func (p *ForgejoProvider) CreateComment(owner, repo string, index int64, body string) (*Comment, error) {
	comment, _, err := p.client.CreateIssueComment(owner, repo, index, forgejo_sdk.CreateIssueCommentOption{
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to comment on PR #%d: %w", index, err)
	}

	return toComment(comment), nil
}

func (p *ForgejoProvider) ListComments(owner, repo string, index int64) ([]*Comment, error) {
	comments, _, err := p.client.ListIssueComments(owner, repo, index, forgejo_sdk.ListIssueCommentOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list comments on PR #%d: %w", index, err)
	}

	result := make([]*Comment, len(comments))
	for i, c := range comments {
		result[i] = toComment(c)
	}
	return result, nil
}

func (p *ForgejoProvider) GetCombinedStatus(owner, repo, ref string) (*CombinedStatus, error) {
	cs, _, err := p.client.GetCombinedStatus(owner, repo, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit status: %w", err)
	}
	if cs == nil {
		return &CombinedStatus{State: "unknown"}, nil
	}

	statuses := make([]*CommitStatus, len(cs.Statuses))
	for i, s := range cs.Statuses {
		statuses[i] = &CommitStatus{
			Context:     s.Context,
			State:       string(s.State),
			Description: s.Description,
			TargetURL:   s.TargetURL,
		}
	}

	return &CombinedStatus{
		State:    string(cs.State),
		Statuses: statuses,
	}, nil
}

// forgejoActionRun mirrors the Forgejo Actions API response for workflow runs.
type forgejoActionRun struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	HTMLURL   string `json:"html_url"`
	CommitSHA string `json:"commit_sha"`
}

type forgejoActionRunList struct {
	WorkflowRuns []forgejoActionRun `json:"workflow_runs"`
}

// forgejoActionTask mirrors the Forgejo /actions/tasks API response.
// Each task corresponds to a single job within a workflow run.
type forgejoActionTask struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	HeadSHA    string `json:"head_sha"`
	RunNumber  int64  `json:"run_number"`
	Status     string `json:"status"`
	WorkflowID string `json:"workflow_id"`
}

type forgejoActionTaskList struct {
	WorkflowRuns []forgejoActionTask `json:"workflow_runs"`
}

func (p *ForgejoProvider) GetCIFailureDetails(owner, repo, sha string) ([]*JobFailure, error) {
	runs, err := p.listActionRuns(owner, repo, sha)
	if err != nil {
		return nil, fmt.Errorf("failed to list action runs: %w", err)
	}

	var failedRuns []forgejoActionRun
	for _, run := range runs {
		if isFailedStatus(run.Status) {
			failedRuns = append(failedRuns, run)
		}
	}
	if len(failedRuns) == 0 {
		return nil, nil
	}

	// Sort by ID for deterministic ordering when multiple runs fail.
	sort.Slice(failedRuns, func(i, j int) bool {
		return failedRuns[i].ID < failedRuns[j].ID
	})

	// Fetch tasks (jobs) for individual job names.
	// Note: Forgejo's public API does not expose per-job log retrieval,
	// so LogTail is always empty for Forgejo tasks.
	tasks, err := p.listActionTasks(owner, repo, sha)
	if err != nil {
		// Fall back to run-level reporting
		failures := make([]*JobFailure, 0, len(failedRuns))
		for _, run := range failedRuns {
			failures = append(failures, &JobFailure{
				WorkflowName: run.Title,
				JobName:      "(could not fetch job details)",
				HTMLURL:      run.HTMLURL,
			})
		}
		return failures, nil
	}

	// Use the first failed run (by ID) as the default URL/workflow name
	// for tasks that don't have a direct run association.
	defaultRun := failedRuns[0]

	failures := make([]*JobFailure, 0, len(tasks))
	for _, task := range tasks {
		if !isFailedStatus(task.Status) {
			continue
		}
		failures = append(failures, &JobFailure{
			WorkflowName: defaultRun.Title,
			JobName:      task.Name,
			HTMLURL:      defaultRun.HTMLURL,
		})
	}

	// If no failed tasks but runs failed, report at run level
	if len(failures) == 0 {
		for _, run := range failedRuns {
			failures = append(failures, &JobFailure{
				WorkflowName: run.Title,
				JobName:      "(check run page for details)",
				HTMLURL:      run.HTMLURL,
			})
		}
	}

	return failures, nil
}

func (p *ForgejoProvider) forgejoGet(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+p.token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (p *ForgejoProvider) listActionRuns(owner, repo, sha string) ([]forgejoActionRun, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/actions/runs?head_sha=%s",
		strings.TrimRight(p.baseURL, "/"), owner, repo, sha)
	var result forgejoActionRunList
	if err := p.forgejoGet(url, &result); err != nil {
		return nil, err
	}
	return result.WorkflowRuns, nil
}

// listActionTasks fetches action tasks for a repo and filters by SHA.
// The /actions/tasks endpoint has no SHA filter, so we filter client-side.
func (p *ForgejoProvider) listActionTasks(owner, repo, sha string) ([]forgejoActionTask, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/actions/tasks?limit=50",
		strings.TrimRight(p.baseURL, "/"), owner, repo)
	var result forgejoActionTaskList
	if err := p.forgejoGet(url, &result); err != nil {
		return nil, err
	}
	var matched []forgejoActionTask
	for _, t := range result.WorkflowRuns {
		if t.HeadSHA == sha {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

func toPullRequest(pr *forgejo_sdk.PullRequest) *PullRequest {
	head := ""
	headSHA := ""
	base := ""
	if pr.Head != nil {
		head = pr.Head.Name
		headSHA = pr.Head.Sha
	}
	if pr.Base != nil {
		base = pr.Base.Name
	}
	return &PullRequest{
		Index:     pr.Index,
		Title:     pr.Title,
		Body:      pr.Body,
		State:     string(pr.State),
		HTMLURL:   pr.HTMLURL,
		Head:      head,
		HeadSHA:   headSHA,
		Base:      base,
		Mergeable: pr.Mergeable,
		Merged:    pr.HasMerged,
	}
}

func toComment(c *forgejo_sdk.Comment) *Comment {
	user := ""
	if c.Poster != nil {
		user = c.Poster.UserName
	}
	return &Comment{
		ID:        c.ID,
		Body:      c.Body,
		User:      user,
		CreatedAt: c.Created,
		HTMLURL:   c.HTMLURL,
	}
}
