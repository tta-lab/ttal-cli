package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// isProtectedBranch returns true if the given branch name is protected by policy.
// The list is intentionally small — extending it requires a code change.
func isProtectedBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

// handleGitPush executes a git push using daemon-held credentials.
// WorkDir may be a ttal worktree or any registered project directory.
// Credentials are injected via GIT_CONFIG env vars — never via URL embedding or keychain.
func handleGitPush(req GitPushRequest) GitPushResponse {
	// Validation order: empty branch → protected-branch policy → credentials
	if req.Branch == "" {
		return GitPushResponse{Error: "branch must not be empty"}
	}
	if req.Force && isProtectedBranch(req.Branch) {
		return GitPushResponse{Error: fmt.Sprintf("force push to %s blocked by ttal policy", req.Branch)}
	}

	// Detect remote URL to pick the right token.
	remoteURL, err := gitutil.RemoteURL(req.WorkDir)
	if err != nil {
		return GitPushResponse{Error: fmt.Sprintf("get remote URL: %v", err)}
	}

	if !gitutil.GitCredEnvHasToken(remoteURL, req.ProjectAlias) {
		return GitPushResponse{Error: fmt.Sprintf("no token for %s (project: %s)", remoteURL, req.ProjectAlias)}
	}
	credEnv := gitutil.GitCredEnv(remoteURL, req.ProjectAlias)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Audit trail for force pushes — not unit-tested; verified via daemon.log in manual smoke.
	if req.Force {
		log.Printf("[daemon] git push --force-with-lease: workdir=%s branch=%s", req.WorkDir, req.Branch)
	}

	cmd := exec.CommandContext(ctx, "git", buildGitPushArgs(req)...)
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

	if !gitutil.GitCredEnvHasToken(remoteURL, req.ProjectAlias) {
		return GitTagResponse{Error: fmt.Sprintf("no token for %s (project: %s)", remoteURL, req.ProjectAlias)}
	}
	credEnv := gitutil.GitCredEnv(remoteURL, req.ProjectAlias)

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

// handleGitPull runs the pull workflow selected by the CLI.
func handleGitPull(req GitPullRequest) GitPullResponse {
	if req.WorkDir == "" {
		return GitPullResponse{Error: "work_dir must not be empty"}
	}
	if req.Branch == "" {
		return GitPullResponse{Error: "branch must not be empty"}
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}
	if req.Mode == "" {
		req.Mode = GitPullModeBranch
	}

	remoteURL, err := gitutil.RemoteURL(req.WorkDir)
	if err != nil {
		return GitPullResponse{Error: fmt.Sprintf("get remote URL: %v", err)}
	}
	if !gitutil.GitCredEnvHasToken(remoteURL, req.ProjectAlias) {
		return GitPullResponse{Error: fmt.Sprintf("no token for %s (project: %s)", remoteURL, req.ProjectAlias)}
	}
	credEnv := gitutil.GitCredEnv(remoteURL, req.ProjectAlias)

	if req.Mode == GitPullModeCleanupMerged {
		if err := ensureBranchNotAheadOfOrigin(req.WorkDir, req.Branch); err != nil {
			return GitPullResponse{Error: err.Error()}
		}
	}

	for _, args := range buildGitPullCommands(req) {
		if err := runGitPullCommand(req.WorkDir, args, credEnv); err != nil {
			return GitPullResponse{Error: err.Error()}
		}
	}

	switch req.Mode {
	case GitPullModeDefault:
		return GitPullResponse{OK: true, Action: GitPullActionPulledDefault}
	case GitPullModeCleanupMerged:
		return GitPullResponse{OK: true, Action: GitPullActionCleanedMergedBranch}
	default:
		return GitPullResponse{OK: true, Action: GitPullActionPulledBranch}
	}
}

// buildGitPushArgs returns the full argv (after "git") for pushing a branch.
// --force-with-lease is appended when req.Force is set. We never emit a raw --force.
func buildGitPushArgs(req GitPushRequest) []string {
	args := []string{"-C", req.WorkDir, "push", "-u", "origin", req.Branch}
	if req.Force {
		args = append(args, "--force-with-lease")
	}
	return args
}

// buildGitPullCommands returns each git argv (after "git") for the selected pull mode.
func buildGitPullCommands(req GitPullRequest) [][]string {
	switch req.Mode {
	case GitPullModeDefault:
		return [][]string{{"-C", req.WorkDir, "pull", "--ff-only", "origin", req.DefaultBranch}}
	case GitPullModeCleanupMerged:
		return [][]string{
			{"-C", req.WorkDir, "switch", req.DefaultBranch},
			{"-C", req.WorkDir, "pull", "--ff-only", "origin", req.DefaultBranch},
			{"-C", req.WorkDir, "branch", "-D", req.Branch},
			{"-C", req.WorkDir, "push", "origin", "--delete", req.Branch},
		}
	default:
		return [][]string{{"-C", req.WorkDir, "pull", "--ff-only", "origin", req.Branch}}
	}
}

func runGitPullCommand(workDir string, args []string, credEnv []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), credEnv...)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if isRemoteDeleteCommand(args) && isMissingRemoteBranchDelete(out.String()) {
			log.Printf("[daemon] remote branch already absent for %s: git %v", workDir, args)
			return nil
		}
		log.Printf("[daemon] git pull workflow failed for %s: git %v: %v — %s", workDir, args, err, out.String())
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return nil
}

func ensureBranchNotAheadOfOrigin(workDir, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	refRange := fmt.Sprintf("origin/%s...%s", branch, branch)
	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "rev-list", "--right-only", "--count", refRange)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"refusing merged-branch cleanup: cannot verify %s is synced with origin/%s: %v\n%s",
			branch,
			branch,
			err,
			strings.TrimSpace(out.String()),
		)
	}

	ahead, err := parseLocalAheadCount(out.String())
	if err != nil {
		return fmt.Errorf("refusing merged-branch cleanup: parse ahead count for %s: %w", branch, err)
	}
	if ahead > 0 {
		return fmt.Errorf(
			"refusing merged-branch cleanup: %s has %d local commit(s) not on origin/%s",
			branch,
			ahead,
			branch,
		)
	}
	return nil
}

func parseLocalAheadCount(output string) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return 0, fmt.Errorf("git rev-list returned %q: %w", strings.TrimSpace(output), err)
	}
	return count, nil
}

func isRemoteDeleteCommand(args []string) bool {
	for i := 0; i < len(args)-2; i++ {
		if args[i] == "push" && args[i+1] == "origin" && args[i+2] == "--delete" {
			return true
		}
	}
	return false
}

func isMissingRemoteBranchDelete(output string) bool {
	return strings.Contains(strings.ToLower(output), "remote ref does not exist")
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
