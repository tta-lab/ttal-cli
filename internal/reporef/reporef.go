package reporef

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var cloneGitRepo = func(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create clone parent: %w", err)
	}
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ResolveOrCloneRepo resolves target as either a bare repo name or an org/repo
// GitHub reference. Bare names only resolve existing unique local clones.
// org/repo targets clone from GitHub when missing locally.
func ResolveOrCloneRepo(target, referencesPath string) (string, error) {
	if org, repo, ok := parseOrgRepo(target); ok {
		repoPath := filepath.Join(referencesPath, "github.com", org, repo)
		if info, err := os.Stat(repoPath); err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("repo path exists but is not a directory: %s", repoPath)
			}
			return repoPath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("checking repo path %s: %w", repoPath, err)
		}

		url := fmt.Sprintf("https://github.com/%s/%s.git", org, repo)
		if err := cloneGitRepo(url, repoPath); err != nil {
			return "", fmt.Errorf("clone %s into %s: %w", url, repoPath, err)
		}
		return repoPath, nil
	}

	return FindClonedRepo(target, referencesPath)
}

func parseOrgRepo(target string) (string, string, bool) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	org, repo := parts[0], parts[1]
	if !isSafePathPart(org) || !isSafePathPart(repo) {
		return "", "", false
	}
	return org, repo, true
}

func isSafePathPart(part string) bool {
	return part != "" && part != "." && part != ".." && !strings.Contains(part, string(filepath.Separator))
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
