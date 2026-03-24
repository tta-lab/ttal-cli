package project

import (
	"os"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// ResolveGitHubToken returns the GitHub token for a project.
// Uses hierarchical project name resolution (same as ResolveProjectPath)
// to find the project's github_token_env field, then calls os.Getenv() to resolve it.
// Falls back to os.Getenv("GITHUB_TOKEN") if override is not configured or empty.
//
// This uses os.Getenv() — consistent with how all daemon tokens are resolved.
// The .env file is loaded into the process once at daemon startup; adding new
// token values to .env requires a daemon restart (same as FORGEJO_TOKEN, etc.).
// The projects.toml file IS re-read on each call (existing store pattern), so
// adding/changing github_token_env on a project takes effect immediately.
func ResolveGitHubToken(projectAlias string) string {
	return resolveGitHubTokenWithStore(projectAlias, NewStore(config.ResolveProjectsPath()))
}

// ResolveGitHubTokenForTeam is like ResolveGitHubToken but reads from the specified team's store.
func ResolveGitHubTokenForTeam(projectAlias, team string) string {
	if team == "" {
		return ResolveGitHubToken(projectAlias)
	}
	return resolveGitHubTokenWithStore(projectAlias, NewStore(config.ResolveProjectsPathForTeam(team)))
}

func resolveGitHubTokenWithStore(projectAlias string, store *Store) string {
	if projectAlias == "" {
		return os.Getenv("GITHUB_TOKEN")
	}
	proj := resolveProjectWithStore(projectAlias, store)
	if proj == nil || proj.GitHubTokenEnv == "" {
		return os.Getenv("GITHUB_TOKEN")
	}
	if token := os.Getenv(proj.GitHubTokenEnv); token != "" {
		return token
	}
	return os.Getenv("GITHUB_TOKEN")
}
