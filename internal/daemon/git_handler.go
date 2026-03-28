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

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
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

// handleGitTag creates a lightweight git tag and pushes it to origin using daemon-held credentials.
// WorkDir is validated against registered ttal project paths to prevent arbitrary repo tagging.
func handleGitTag(req GitTagRequest) GitTagResponse {
	if req.Tag == "" {
		return GitTagResponse{Error: "tag must not be empty"}
	}
	if req.WorkDir == "" {
		return GitTagResponse{Error: "work_dir must not be empty"}
	}

	// Security: validate WorkDir is a registered ttal project path (exact match).
	if !isRegisteredProjectPath(req.WorkDir) {
		return GitTagResponse{Error: "tag only allowed for registered ttal projects"}
	}

	// Detect remote URL to pick the right token.
	remoteURL, err := getRemoteURL(req.WorkDir)
	if err != nil {
		return GitTagResponse{Error: fmt.Sprintf("get remote URL: %v", err)}
	}

	token := tokenForHost(remoteURL)
	if token == "" {
		return GitTagResponse{Error: fmt.Sprintf("no token available for host: %s", remoteURL)}
	}

	// Create the tag locally. "--" prevents tag names from being parsed as flags.
	// If tag already exists, git exits 128 — we surface this as an error (no overwrite).
	ctxTag, cancelTag := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelTag()

	tagCmd := exec.CommandContext(ctxTag, "git", "-C", req.WorkDir, "tag", "--", req.Tag)
	var tagOut bytes.Buffer
	tagCmd.Stdout = &tagOut
	tagCmd.Stderr = &tagOut
	if err := tagCmd.Run(); err != nil {
		return GitTagResponse{Error: fmt.Sprintf("git tag: %v\n%s", err, strings.TrimSpace(tagOut.String()))}
	}

	// Push the tag to origin with credential injection.
	ctxPush, cancelPush := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelPush()

	pushCmd := exec.CommandContext(ctxPush, "git", "-C", req.WorkDir, "push", "origin", "--", req.Tag)
	pushCmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=credential.helper",
		fmt.Sprintf("GIT_CONFIG_VALUE_1=!f(){ echo username=x-access-token; echo password='%s'; }; f", token),
	)
	var pushOut bytes.Buffer
	pushCmd.Stdout = &pushOut
	pushCmd.Stderr = &pushOut

	if err := pushCmd.Run(); err != nil {
		// Tag was created locally but push failed — delete the local tag to avoid stale state.
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = exec.CommandContext(cleanCtx, "git", "-C", req.WorkDir, "tag", "-d", req.Tag).Run()

		log.Printf("[daemon] git tag push failed for %s: %v — %s", req.WorkDir, err, pushOut.String())
		return GitTagResponse{Error: fmt.Sprintf("git push tag: %v\n%s", err, strings.TrimSpace(pushOut.String()))}
	}

	log.Printf("[daemon] git tag ok: %s → %s", req.Tag, req.WorkDir)
	return GitTagResponse{OK: true}
}

// isRegisteredProjectPath checks if the given path is a registered ttal project path.
// Uses exact match after filepath.Clean to prevent path-traversal attacks.
func isRegisteredProjectPath(path string) bool {
	cleanPath := filepath.Clean(path)
	store := project.NewStore(config.ResolveProjectsPath())
	projects, err := store.List(false)
	if err != nil {
		return false
	}
	for _, p := range projects {
		if filepath.Clean(p.Path) == cleanPath {
			return true
		}
	}
	return false
}
