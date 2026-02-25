package gitprovider

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v69/github"
)

type GitHubProvider struct {
	client *github.Client
}

func NewGitHubProvider() (Provider, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	client := github.NewClient(nil).WithAuthToken(token)
	return &GitHubProvider{client: client}, nil
}

func (p *GitHubProvider) CreatePR(owner, repo, head, base, title, body string) (*PullRequest, error) {
	pr, _, err := p.client.PullRequests.Create(context.Background(), owner, repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	return toGitHubPullRequest(pr), nil
}

func (p *GitHubProvider) EditPR(owner, repo string, index int64, title, body string) (*PullRequest, error) {
	opt := &github.PullRequest{}
	if title != "" {
		opt.Title = &title
	}
	if body != "" {
		opt.Body = &body
	}

	pr, _, err := p.client.PullRequests.Edit(context.Background(), owner, repo, int(index), opt)
	if err != nil {
		return nil, fmt.Errorf("failed to edit PR #%d: %w", index, err)
	}

	return toGitHubPullRequest(pr), nil
}

func (p *GitHubProvider) GetPR(owner, repo string, index int64) (*PullRequest, error) {
	pr, _, err := p.client.PullRequests.Get(context.Background(), owner, repo, int(index))
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %w", index, err)
	}

	return toGitHubPullRequest(pr), nil
}

func (p *GitHubProvider) MergePR(owner, repo string, index int64, deleteBranch bool) error {
	squash := "squash"
	result, _, err := p.client.PullRequests.Merge(context.Background(), owner, repo, int(index),
		"", &github.PullRequestOptions{
			MergeMethod: squash,
		})
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", index, err)
	}
	if !result.GetMerged() {
		return fmt.Errorf("PR #%d merge was rejected: %s", index, result.GetMessage())
	}

	if deleteBranch {
		pr, _, err := p.client.PullRequests.Get(context.Background(), owner, repo, int(index))
		if err != nil {
			return fmt.Errorf("failed to get PR #%d for branch deletion: %w", index, err)
		}
		if pr.Head != nil && pr.Head.Ref != nil {
			_, err = p.client.Git.DeleteRef(context.Background(), owner, repo, "heads/"+*pr.Head.Ref)
			if err != nil {
				return fmt.Errorf("failed to delete branch: %w", err)
			}
		}
	}

	return nil
}

func (p *GitHubProvider) CreateComment(owner, repo string, index int64, body string) (*Comment, error) {
	comment, _, err := p.client.Issues.CreateComment(context.Background(), owner, repo, int(index), &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to comment on PR #%d: %w", index, err)
	}

	return toGitHubComment(comment), nil
}

func (p *GitHubProvider) ListComments(owner, repo string, index int64) ([]*Comment, error) {
	comments, _, err := p.client.Issues.ListComments(context.Background(), owner, repo, int(index), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments on PR #%d: %w", index, err)
	}

	result := make([]*Comment, len(comments))
	for i, c := range comments {
		result[i] = toGitHubComment(c)
	}
	return result, nil
}

func (p *GitHubProvider) GetCombinedStatus(owner, repo, ref string) (*CombinedStatus, error) {
	cs, _, err := p.client.Repositories.GetCombinedStatus(context.Background(), owner, repo, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit status: %w", err)
	}
	if cs == nil {
		return &CombinedStatus{State: "unknown"}, nil
	}

	statuses := make([]*CommitStatus, len(cs.Statuses))
	for i, s := range cs.Statuses {
		statuses[i] = &CommitStatus{
			Context:     s.GetContext(),
			State:       s.GetState(),
			Description: s.GetDescription(),
			TargetURL:   s.GetTargetURL(),
		}
	}

	return &CombinedStatus{
		State:    cs.GetState(),
		Statuses: statuses,
	}, nil
}

func toGitHubPullRequest(pr *github.PullRequest) *PullRequest {
	head := ""
	headSHA := ""
	base := ""
	if pr.Head != nil {
		if pr.Head.Ref != nil {
			head = *pr.Head.Ref
		}
		if pr.Head.SHA != nil {
			headSHA = *pr.Head.SHA
		}
	}
	if pr.Base != nil && pr.Base.Ref != nil {
		base = *pr.Base.Ref
	}
	mergeable := true
	if pr.Mergeable != nil {
		mergeable = *pr.Mergeable
	}
	return &PullRequest{
		Index:     int64(pr.GetNumber()),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		State:     pr.GetState(),
		HTMLURL:   pr.GetHTMLURL(),
		Head:      head,
		HeadSHA:   headSHA,
		Base:      base,
		Mergeable: mergeable,
		Merged:    pr.GetMerged(),
	}
}

func toGitHubComment(c *github.IssueComment) *Comment {
	user := ""
	if c.User != nil {
		user = c.User.GetLogin()
	}
	return &Comment{
		ID:        c.GetID(),
		Body:      c.GetBody(),
		User:      user,
		CreatedAt: c.GetCreatedAt().Time,
		HTMLURL:   c.GetHTMLURL(),
	}
}
