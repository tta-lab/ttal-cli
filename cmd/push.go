package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/project"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push current branch to origin via daemon (no credentials needed in worker)",
	Long: `Pushes the current git branch to origin through the daemon.
The daemon injects credentials — workers don't need tokens in their environment.

Equivalent to "git push -u origin <branch>" but credential-safe for worker sessions.

Examples:
  ttal push`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		branch, err := currentBranch(workDir)
		if err != nil {
			return fmt.Errorf("get current branch: %w", err)
		}

		projectAlias := resolveAliasFromPath(workDir)
		resp, err := daemon.GitPush(daemon.GitPushRequest{
			WorkDir:      workDir,
			Branch:       branch,
			ProjectAlias: projectAlias,
		})
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("push failed: %s", resp.Error)
		}

		fmt.Printf("Pushed %s → origin/%s\n", branch, branch)
		return nil
	},
}

// currentBranch returns the current git branch name for the given working directory.
func currentBranch(workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "git", "-C", workDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("not on a named branch (detached HEAD or empty repo)")
	}
	return branch, nil
}

// storeFactoryFn is injectable for testing. Defaults to creating a store from
// the real projects.toml path.
var storeFactoryFn = func() *project.Store {
	return project.NewStore(config.ResolveProjectsPath())
}

// resolveAliasFromPath resolves the project alias for the given working directory
// by scanning registered projects for a path match. Falls back to extracting the
// alias suffix from a worktree directory name (e.g. ~/.ttal/worktrees/<uuid>-<alias>).
// Returns empty string if no match is found — callers fall back to GITHUB_TOKEN.
func resolveAliasFromPath(workDir string) string {
	store := storeFactoryFn()
	projects, err := store.List(false)
	if err != nil {
		return ""
	}
	cleanWork := filepath.Clean(workDir)
	for _, p := range projects {
		cleanProj := filepath.Clean(p.Path)
		if cleanWork == cleanProj || strings.HasPrefix(cleanWork, cleanProj+string(filepath.Separator)) {
			return p.Alias
		}
	}
	// Worktree path: ~/.ttal/worktrees/<uuid8>-<alias>/...
	// Extract alias from the directory name suffix after the UUID prefix.
	base := filepath.Base(cleanWork)
	if idx := strings.Index(base, "-"); idx >= 0 {
		candidate := base[idx+1:]
		for _, p := range projects {
			if p.Alias == candidate {
				return candidate
			}
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
