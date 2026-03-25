package ask

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const repoOpTimeout = 5 * time.Minute

// ResolveRepoRef converts a repo reference to a clone URL and local path.
// Supports full URLs, "org/repo" shorthands (→ GitHub), and bare repo names
// for repos that are already cloned locally in the references directory.
func ResolveRepoRef(ref, referencesPath string) (cloneURL, localPath string, err error) {
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		u, parseErr := url.Parse(ref)
		if parseErr != nil {
			return "", "", fmt.Errorf("invalid URL: %w", parseErr)
		}
		repoPath := strings.TrimPrefix(u.Path, "/")
		repoPath = strings.TrimSuffix(repoPath, "/")
		repoPath = strings.TrimSuffix(repoPath, ".git")
		localPath = filepath.Join(referencesPath, u.Host, repoPath)
		cloneURL = ref
	} else if strings.Contains(ref, "/") {
		// Shorthand: must be "org/repo" format (exactly one slash)
		ref = strings.TrimSuffix(ref, "/")
		ref = strings.TrimSuffix(ref, ".git")
		parts := strings.Split(ref, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf(
				"invalid repo shorthand %q: expected \"org/repo\" (e.g. woodpecker-ci/woodpecker) or a full URL",
				ref,
			)
		}
		cloneURL = "https://github.com/" + ref
		localPath = filepath.Join(referencesPath, "github.com", ref)
	} else {
		// Bare name: scan already-cloned repos for a match.
		// Only works for repos that are already cloned locally.
		localPath, err = FindClonedRepo(ref, referencesPath)
		if err != nil {
			return "", "", err
		}
		// Derive cloneURL from local path for EnsureRepo's git-pull path.
		// Note: bare-name repos are always already cloned, so EnsureRepo will
		// only use this for "git pull", never "git clone".
		rel, relErr := filepath.Rel(referencesPath, localPath)
		if relErr != nil {
			return "", "", fmt.Errorf("computing repo clone URL from %s: %w", localPath, relErr)
		}
		cloneURL = "https://" + rel
	}
	return cloneURL, localPath, nil
}

// EnsureRepo clones the repo if it doesn't exist, or pulls if it does.
func EnsureRepo(cloneURL, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), repoOpTimeout)
	defer cancel()

	if dirExists(localPath) {
		fmt.Fprintf(os.Stderr, "Updating %s...\n", filepath.Base(localPath))
		cmd := exec.CommandContext(ctx, "git", "-C", localPath, "pull", "--ff-only")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull failed in %s: %w", localPath, err)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Cloning %s...\n", cloneURL)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, localPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %s into %s: %w", cloneURL, localPath, err)
	}
	return nil
}

// scanHostDir scans host/org/repo under hostPath, collecting paths where the
// final directory component equals name.
func scanHostDir(name, hostPath string) []string {
	var matches []string
	orgs, err := os.ReadDir(hostPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", hostPath, err)
		return nil
	}
	for _, org := range orgs {
		if !org.IsDir() {
			continue
		}
		orgPath := filepath.Join(hostPath, org.Name())
		repos, err := os.ReadDir(orgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", orgPath, err)
			continue
		}
		for _, repo := range repos {
			if repo.IsDir() && repo.Name() == name {
				matches = append(matches, filepath.Join(orgPath, repo.Name()))
			}
		}
	}
	return matches
}

// FindClonedRepo scans the references directory for an already-cloned repo
// matching the bare name (case-sensitive). Returns the local path if exactly
// one match is found. Errors with disambiguation list on multiple matches.
func FindClonedRepo(name, referencesPath string) (string, error) {
	var matches []string

	hosts, err := os.ReadDir(referencesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"repo %q not found as org/repo; references directory does not exist at %s",
				name, referencesPath,
			)
		}
		return "", fmt.Errorf(
			"repo %q not found as org/repo; could not read references directory %s: %w",
			name, referencesPath, err,
		)
	}

	for _, host := range hosts {
		if !host.IsDir() {
			continue
		}
		hostPath := filepath.Join(referencesPath, host.Name())
		matches = append(matches, scanHostDir(name, hostPath)...)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"repo %q not found locally; use org/repo format (e.g. charmbracelet/%s) to clone it",
			name, name,
		)
	case 1:
		return matches[0], nil
	default:
		var options []string
		for _, m := range matches {
			rel, relErr := filepath.Rel(referencesPath, m)
			if relErr != nil {
				options = append(options, m) // fallback to absolute path
				continue
			}
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			if len(parts) == 2 {
				options = append(options, parts[1])
			} else {
				options = append(options, rel)
			}
		}
		return "", fmt.Errorf(
			"ambiguous repo name %q matches multiple repos:\n  %s\n\nSpecify org/repo to disambiguate",
			name, strings.Join(options, "\n  "),
		)
	}
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
