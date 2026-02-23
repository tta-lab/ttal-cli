package gitprovider

import (
	"context"
	"fmt"
	urlpkg "net/url"
	"os/exec"
	"strings"
	"time"
)

const remoteTimeout = 10 * time.Second

type RepoInfo struct {
	Owner    string
	Repo     string
	Provider ProviderType
	Host     string
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
	return ParseRemoteURL(remoteURL)
}

func ParseRemoteURL(remoteURL string) (*RepoInfo, error) {
	if strings.HasPrefix(remoteURL, "git@") {
		return parseSSHShorthand(remoteURL)
	}
	if strings.HasPrefix(remoteURL, "ssh://") {
		return parseSSHProtocol(remoteURL)
	}
	if strings.HasPrefix(remoteURL, "http://") || strings.HasPrefix(remoteURL, "https://") {
		return parseHTTP(remoteURL)
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

func parseSSHProtocol(url string) (*RepoInfo, error) {
	u, err := urlpkg.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH URL: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		host = u.Host
	}
	if strings.Contains(host, "@") {
		parts := strings.SplitN(host, "@", 2)
		host = parts[1]
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

func parseHTTP(url string) (*RepoInfo, error) {
	u, err := urlpkg.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP URL: %w", err)
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

func NewProvider(info *RepoInfo) (Provider, error) {
	switch info.Provider {
	case ProviderGitHub:
		return NewGitHubProvider()
	case ProviderForgejo:
		return NewForgejoProvider("https://" + info.Host)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", info.Provider)
	}
}
