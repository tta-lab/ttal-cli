package forgejo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

const cmdTimeout = 10 * time.Second

var (
	client     *forgejo_sdk.Client
	clientOnce sync.Once
	clientErr  error
)

// Client returns a lazily-initialized Forgejo SDK client.
// Reads FORGEJO_URL and FORGEJO_TOKEN from environment.
func Client() (*forgejo_sdk.Client, error) {
	clientOnce.Do(func() {
		url := os.Getenv("FORGEJO_URL")
		token := os.Getenv("FORGEJO_TOKEN")
		if token == "" {
			token = os.Getenv("FORGEJO_ACCESS_TOKEN")
		}

		if url == "" {
			clientErr = fmt.Errorf("FORGEJO_URL environment variable is required")
			return
		}
		if token == "" {
			clientErr = fmt.Errorf("FORGEJO_TOKEN or FORGEJO_ACCESS_TOKEN environment variable is required")
			return
		}

		client, clientErr = forgejo_sdk.NewClient(url, forgejo_sdk.SetToken(token))
	})
	return client, clientErr
}

// IsPRMerged checks if a pull request has been merged.
func IsPRMerged(owner, repo string, index int64) (bool, error) {
	c, err := Client()
	if err != nil {
		return false, err
	}

	pr, _, err := c.GetPullRequest(owner, repo, index)
	if err != nil {
		return false, fmt.Errorf("failed to get PR #%d: %w", index, err)
	}

	return pr.HasMerged, nil
}

// GetPR fetches a pull request by index.
func GetPR(owner, repo string, index int64) (*forgejo_sdk.PullRequest, error) {
	c, err := Client()
	if err != nil {
		return nil, err
	}

	pr, _, err := c.GetPullRequest(owner, repo, index)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %w", index, err)
	}

	return pr, nil
}

// Git remote URL patterns
var (
	// git@host:owner/repo.git
	sshShorthandRe = regexp.MustCompile(`^git@[^:]+:([^/]+)/([^/]+?)(?:\.git)?$`)
	// ssh://git@host/owner/repo.git
	sshProtocolRe = regexp.MustCompile(`^ssh://[^/]+/([^/]+)/([^/]+?)(?:\.git)?$`)
	// https://host/owner/repo.git
	httpsRe = regexp.MustCompile(`^https?://[^/]+/([^/]+)/([^/]+?)(?:\.git)?$`)
)

// ParseRepoInfo extracts owner and repo name from the git remote URL of a project.
func ParseRepoInfo(workDir string) (owner, repo string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(out))
	return parseRemoteURL(remoteURL)
}

func parseRemoteURL(url string) (owner, repo string, err error) {
	for _, re := range []*regexp.Regexp{sshShorthandRe, sshProtocolRe, httpsRe} {
		if m := re.FindStringSubmatch(url); m != nil {
			return m[1], m[2], nil
		}
	}
	return "", "", fmt.Errorf("could not parse git remote URL: %s", url)
}
