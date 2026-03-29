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
	"github.com/tta-lab/ttal-cli/internal/gitutil"
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
	remoteURL, err := gitutil.RemoteURL(req.WorkDir)
	if err != nil {
		return GitPushResponse{Error: fmt.Sprintf("get remote URL: %v", err)}
	}

	credEnv := gitutil.GitCredEnv(remoteURL, req.ProjectAlias)
	if len(credEnv) == 1 {
		// Only GIT_TERMINAL_PROMPT=0, no token available — fail early with clear error.
		return GitPushResponse{Error: fmt.Sprintf("no token available for host: %s (project: %s)", remoteURL, req.ProjectAlias)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", req.WorkDir, "push", "-u", "origin", req.Branch)
	cmd.Env = append(os.Environ(), credEnv...)

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
	registered, err := isRegisteredProjectPath(req.WorkDir)
	if err != nil {
		return GitTagResponse{Error: fmt.Sprintf("load project registry: %v", err)}
	}
	if !registered {
		return GitTagResponse{Error: "tag only allowed for registered ttal projects"}
	}

	// Detect remote URL to pick the right token.
	remoteURL, err := gitutil.RemoteURL(req.WorkDir)
	if err != nil {
		return GitTagResponse{Error: fmt.Sprintf("get remote URL: %v", err)}
	}

	credEnv := gitutil.GitCredEnv(remoteURL, req.ProjectAlias)
	if len(credEnv) == 1 {
		// Only GIT_TERMINAL_PROMPT=0, no token available — fail early with clear error.
		return GitTagResponse{Error: fmt.Sprintf("no token available for host: %s (project: %s)", remoteURL, req.ProjectAlias)}
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
	pushCmd.Env = append(os.Environ(), credEnv...)
	var pushOut bytes.Buffer
	pushCmd.Stdout = &pushOut
	pushCmd.Stderr = &pushOut

	if err := pushCmd.Run(); err != nil {
		// Tag was created locally but push failed — delete the local tag to avoid stale state.
		// "--" prevents tag names like "-v1.0.0" from being parsed as flags.
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		cleanCmd := exec.CommandContext(cleanCtx, "git", "-C", req.WorkDir, "tag", "-d", "--", req.Tag)
		if cleanErr := cleanCmd.Run(); cleanErr != nil {
			log.Printf("[daemon] git tag cleanup failed (tag %s may be stale in %s): %v",
				req.Tag, req.WorkDir, cleanErr)
		} else {
			log.Printf("[daemon] git tag rolled back: deleted local tag %s", req.Tag)
		}

		log.Printf("[daemon] git tag push failed for %s: %v — %s", req.WorkDir, err, pushOut.String())
		return GitTagResponse{Error: fmt.Sprintf("git push tag: %v\n%s", err, strings.TrimSpace(pushOut.String()))}
	}

	log.Printf("[daemon] git tag ok: %s → %s", req.Tag, req.WorkDir)
	return GitTagResponse{OK: true}
}

// isRegisteredProjectPath checks if the given path is a registered ttal project path.
// Uses exact match after filepath.Clean to prevent path-traversal attacks.
// Returns (false, err) when the project store cannot be read, so callers can surface
// config errors instead of the misleading "not a registered project" message.
func isRegisteredProjectPath(path string) (bool, error) {
	cleanPath := filepath.Clean(path)
	store := project.NewStore(config.ResolveProjectsPath())
	projects, err := store.List(false)
	if err != nil {
		return false, err
	}
	for _, p := range projects {
		if filepath.Clean(p.Path) == cleanPath {
			return true, nil
		}
	}
	return false, nil
}
