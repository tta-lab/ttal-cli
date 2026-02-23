package gitprovider

import (
	"fmt"
	"os"
	"sync"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

type ForgejoProvider struct {
	client *forgejo_sdk.Client
}

var (
	forgejoTokenOnce sync.Once
	forgejoToken     string
)

func getForgejoToken() string {
	forgejoTokenOnce.Do(func() {
		forgejoToken = os.Getenv("FORGEJO_TOKEN")
		if forgejoToken == "" {
			forgejoToken = os.Getenv("FORGEJO_ACCESS_TOKEN")
		}
	})
	return forgejoToken
}

func NewForgejoProvider(_ string) (Provider, error) {
	token := getForgejoToken()
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

	return &ForgejoProvider{client: client}, nil
}

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

func toPullRequest(pr *forgejo_sdk.PullRequest) *PullRequest {
	head := ""
	base := ""
	if pr.Head != nil {
		head = pr.Head.Name
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
