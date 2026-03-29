package gitutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/project"
)

// GitCredEnv returns environment variables for git network operations.
// It ALWAYS includes GIT_TERMINAL_PROMPT=0 to suppress interactive prompts.
// If a token is available for the remote host, it also injects a one-shot
// credential helper that clears osxkeychain and provides the token.
//
// projectAlias is optional — when provided, per-project github_token_env
// overrides are respected via project.ResolveGitHubToken().
//
// The token is passed via GIT_TOKEN_INJECT rather than interpolated into the
// shell string, which avoids breakage if a token ever contains shell metacharacters.
//
// Use GitCredEnvHasToken to check whether credentials were injected.
func GitCredEnv(remoteURL, projectAlias string) []string {
	// Always suppress interactive prompts — this prevents the hang bug
	// even when no token is configured.
	env := []string{"GIT_TERMINAL_PROMPT=0"}

	token := tokenForRemote(remoteURL, projectAlias)
	if token == "" {
		return env
	}

	return append(env,
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=credential.helper",
		"GIT_CONFIG_VALUE_1=!f(){ echo username=x-access-token; echo password=$GIT_TOKEN_INJECT; }; f",
		"GIT_TOKEN_INJECT="+token,
	)
}

// GitCredEnvHasToken reports whether GitCredEnv would inject a token for the
// given remote URL and project alias. Use this instead of checking len(GitCredEnv(...))
// to guard critical operations that must fail fast without a token.
func GitCredEnvHasToken(remoteURL, projectAlias string) bool {
	return tokenForRemote(remoteURL, projectAlias) != ""
}

// RemoteURL returns the origin remote URL for the given directory.
func RemoteURL(dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// tokenForRemote selects the appropriate token based on remote URL and project alias.
// For GitHub repos: uses project.ResolveGitHubToken (respects github_token_env override).
// For non-GitHub repos: uses FORGEJO_TOKEN.
func tokenForRemote(remoteURL, projectAlias string) string {
	if strings.Contains(remoteURL, "github.com") {
		return project.ResolveGitHubToken(projectAlias)
	}
	return os.Getenv("FORGEJO_TOKEN")
}
