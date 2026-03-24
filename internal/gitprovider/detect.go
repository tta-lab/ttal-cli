package gitprovider

import (
	"context"
	"fmt"
	urlpkg "net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const remoteTimeout = 10 * time.Second

type RepoInfo struct {
	Owner         string
	Repo          string
	Provider      ProviderType
	Host          string
	DefaultBranch string
}

// PRURL constructs the full web URL for a pull request.
func (r *RepoInfo) PRURL(prID string) string {
	var baseURL, prSegment string
	switch r.Provider {
	case ProviderGitHub:
		baseURL = "https://github.com"
		prSegment = "pull"
	default:
		baseURL = os.Getenv("FORGEJO_URL")
		if baseURL == "" {
			baseURL = "https://" + r.Host
		}
		prSegment = "pulls"
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s", baseURL, r.Owner, r.Repo, prSegment, prID)
}

func DetectProvider(workDir string) (*RepoInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(out))
	info, err := ParseRemoteURL(remoteURL)
	if err != nil {
		return nil, err
	}

	info.DefaultBranch = detectDefaultBranch(workDir)
	return info, nil
}

func detectDefaultBranch(workDir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "symbolic-ref", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}

	// Output is like "refs/remotes/origin/main" — extract the branch name
	ref := strings.TrimSpace(string(out))
	const prefix = "refs/remotes/origin/"
	if !strings.HasPrefix(ref, prefix) {
		return "main"
	}
	return ref[len(prefix):]
}

func ParseRemoteURL(remoteURL string) (*RepoInfo, error) {
	if strings.HasPrefix(remoteURL, "git@") {
		return parseSSHShorthand(remoteURL)
	}
	if strings.HasPrefix(remoteURL, "ssh://") ||
		strings.HasPrefix(remoteURL, "http://") ||
		strings.HasPrefix(remoteURL, "https://") {
		return parseURL(remoteURL)
	}
	return nil, fmt.Errorf("could not parse git remote URL: %s", remoteURL)
}

func parseSSHShorthand(url string) (*RepoInfo, error) {
	colonIdx := strings.Index(url, ":")
	if colonIdx == -1 {
		return nil, fmt.Errorf("invalid SSH shorthand URL: %s", url)
	}
	host := url[4:colonIdx]
	path := url[colonIdx+1:]

	owner, repo, err := splitPath(path)
	if err != nil {
		return nil, err
	}
	return &RepoInfo{
		Owner:    owner,
		Repo:     repo,
		Provider: detectProviderFromHost(host),
		Host:     host,
	}, nil
}

func parseURL(raw string) (*RepoInfo, error) {
	u, err := urlpkg.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		host = u.Host
	}

	owner, repo, err := splitPath(strings.TrimPrefix(u.Path, "/"))
	if err != nil {
		return nil, err
	}
	return &RepoInfo{
		Owner:    owner,
		Repo:     repo,
		Provider: detectProviderFromHost(host),
		Host:     host,
	}, nil
}

func splitPath(path string) (owner, repo string, err error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository path: %s (expected owner/repo)", path)
	}
	return parts[0], parts[1], nil
}

func detectProviderFromHost(host string) ProviderType {
	host = strings.ToLower(host)
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		return ProviderGitHub
	}
	return ProviderForgejo
}

// NewProviderByNameWithToken creates a provider by name with an optional GitHub token override.
// Forgejo ignores the githubToken parameter.
func NewProviderByNameWithToken(name, githubToken string) (Provider, error) {
	switch ProviderType(name) {
	case ProviderForgejo:
		return NewForgejoProvider()
	case ProviderGitHub:
		return NewGitHubProviderWithToken(githubToken)
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func NewProviderByName(name string) (Provider, error) {
	return NewProviderByNameWithToken(name, "")
}

// NewProviderWithToken creates a provider from RepoInfo with an optional GitHub token override.
// Forgejo ignores the githubToken parameter.
func NewProviderWithToken(info *RepoInfo, githubToken string) (Provider, error) {
	switch info.Provider {
	case ProviderGitHub:
		return NewGitHubProviderWithToken(githubToken)
	case ProviderForgejo:
		return NewForgejoProvider()
	default:
		return nil, fmt.Errorf("unsupported provider: %s", info.Provider)
	}
}

func NewProvider(info *RepoInfo) (Provider, error) {
	return NewProviderWithToken(info, "")
}
