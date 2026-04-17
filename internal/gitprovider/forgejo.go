package gitprovider

import (
	"fmt"
	"os"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

type ForgejoProvider struct {
	client *forgejo_sdk.Client
}

func NewForgejoProvider(host string) (Provider, error) {
	if host == "" {
		return nil, fmt.Errorf("host is required for Forgejo provider")
	}

	token := os.Getenv("FORGEJO_TOKEN")
	if token == "" {
		token = os.Getenv("FORGEJO_ACCESS_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("FORGEJO_TOKEN or FORGEJO_ACCESS_TOKEN environment variable is required")
	}

	url := host
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + host
	}

	client, err := forgejo_sdk.NewClient(url, forgejo_sdk.SetToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to create Forgejo client: %w", err)
	}

	return &ForgejoProvider{client: client}, nil
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

// GetCIFailureDetails fetches CI failure details via Woodpecker CI API.
// Forgejo's native Actions API does not provide useful error info,
// so we use Woodpecker's API directly for failure details and step logs.
//
// Note: this method requires WOODPECKER_URL and WOODPECKER_TOKEN to be set
// independently of the Forgejo credentials — Woodpecker is a separate service.
func (p *ForgejoProvider) GetCIFailureDetails(owner, repo, sha string) ([]*JobFailure, error) {
	wc, err := NewWoodpeckerClient()
	if err != nil {
		return nil, fmt.Errorf("woodpecker client: %w (set WOODPECKER_URL and WOODPECKER_TOKEN)", err)
	}
	return wc.GetFailureDetails(owner, repo, sha)
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
