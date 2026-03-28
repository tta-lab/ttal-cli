package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// handleGitPush executes a git push from a worktree using daemon-held credentials.
// WorkDir is validated to be under ~/.ttal/worktrees/ to prevent arbitrary repo pushes.
// Credentials are injected via GIT_CONFIG env vars — never via URL embedding or keychain.
func handleGitPush(req GitPushRequest) GitPushResponse {
	home, err := os.UserHomeDir()
	if err != nil {
		return GitPushResponse{Error: fmt.Sprintf("resolve home dir: %v", err)}
	}

	// Security: only allow pushes from ttal-managed worktrees.
	// worktreesBase has a trailing separator so HasPrefix correctly rejects:
	//   - adjacent directories (worktrees-evil/) — no separator match
	//   - the base directory itself (worktrees/) — cleanPath has no trailing separator
	worktreesBase := filepath.Join(home, ".ttal", "worktrees") + string(filepath.Separator)
	cleanPath := filepath.Clean(req.WorkDir)
	if !strings.HasPrefix(cleanPath, worktreesBase) {
		return GitPushResponse{Error: "push only allowed from ttal worktrees"}
	}

	if req.Branch == "" {
		return GitPushResponse{Error: "branch must not be empty"}
	}

	// Detect remote URL to pick the right token.
	remoteURL, err := getRemoteURL(req.WorkDir)
	if err != nil {
		return GitPushResponse{Error: fmt.Sprintf("get remote URL: %v", err)}
	}

	token := tokenForHost(remoteURL)
	if token == "" {
		return GitPushResponse{Error: fmt.Sprintf("no token available for host: %s", remoteURL)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", req.WorkDir, "push", "-u", "origin", req.Branch)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // never prompt — fail immediately if credentials are wrong
		// Two credential.helper entries: the first (empty) clears any helpers inherited from
		// ~/.gitconfig (e.g. osxkeychain), so git doesn't fall through to stored credentials
		// for a different account. The second installs our inline token helper.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=", // clear inherited helpers (osxkeychain etc.)
		"GIT_CONFIG_KEY_1=credential.helper",
		// Single-quote the password to prevent shell metacharacter injection.
		// Git tokens are alphanumeric+hyphen — cannot contain single quotes — so this is safe.
		fmt.Sprintf("GIT_CONFIG_VALUE_1=!f(){ echo username=x-access-token; echo password='%s'; }; f", token),
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		log.Printf("[daemon] git push failed for %s: %v — %s", req.WorkDir, err, out.String())
		return GitPushResponse{Error: fmt.Sprintf("git push: %v\n%s", err, strings.TrimSpace(out.String()))}
	}

	log.Printf("[daemon] git push ok: %s → %s", req.WorkDir, req.Branch)
	return GitPushResponse{OK: true}
}

// getRemoteURL returns the remote URL for origin in the given worktree.
func getRemoteURL(workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "git", "-C", workDir, "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// tokenForHost selects the appropriate token based on the remote URL.
// Returns empty string if no matching token is configured.
func tokenForHost(remoteURL string) string {
	switch {
	case strings.Contains(remoteURL, "github.com"):
		return os.Getenv("GITHUB_TOKEN")
	default:
		// All non-GitHub remotes (Forgejo, Gitea, etc.) use FORGEJO_TOKEN.
		return os.Getenv("FORGEJO_TOKEN")
	}
}
