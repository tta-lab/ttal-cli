package scaffold

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const TemplatesRepo = "https://github.com/tta-lab/ttal-templates.git"

// ScaffoldInfo holds metadata parsed from a scaffold's README.md.
type ScaffoldInfo struct {
	Dir         string // directory name (e.g. "basic", "full-markdown")
	Name        string // display name from README heading
	Description string // first paragraph after heading
	Agents      string // comma-separated agent names (from subdirectories)
	InstallHint string // optional install instructions (from frontmatter)
}

// Apply copies a scaffold and the shared docs/ directory into the workspace.
func Apply(repoDir, scaffoldName, workspace string) error {
	scaffoldDir := filepath.Join(repoDir, scaffoldName)

	// Validate resolved path stays under repoDir (prevent path traversal).
	if rel, err := filepath.Rel(repoDir, scaffoldDir); err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid scaffold name: %q", scaffoldName)
	}

	if info, err := os.Stat(scaffoldDir); err != nil || !info.IsDir() {
		scaffolds, listErr := List(repoDir)
		if listErr != nil {
			return fmt.Errorf("scaffold %q not found and cannot list available scaffolds: %w",
				scaffoldName, listErr)
		}
		names := make([]string, len(scaffolds))
		for i, s := range scaffolds {
			names[i] = s.Dir
		}
		return fmt.Errorf("scaffold %q not found (available: %s)", scaffoldName, strings.Join(names, ", "))
	}

	if err := copyDir(scaffoldDir, workspace); err != nil {
		return fmt.Errorf("copy scaffold: %w", err)
	}

	// Copy shared docs/ if it exists
	docsDir := filepath.Join(repoDir, "docs")
	if info, err := os.Stat(docsDir); err == nil && info.IsDir() {
		if err := copyDir(docsDir, filepath.Join(workspace, "docs")); err != nil {
			return fmt.Errorf("copy docs: %w", err)
		}
	}

	return nil
}

// List returns available scaffolds with metadata parsed from README.md.
// A scaffold is a directory containing config.toml.
func List(repoDir string) ([]ScaffoldInfo, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}

	scaffolds := make([]ScaffoldInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoDir, e.Name(), "config.toml")); err != nil {
			continue
		}

		info := ScaffoldInfo{Dir: e.Name()}

		// Parse README.md for metadata
		readmePath := filepath.Join(repoDir, e.Name(), "README.md")
		parseReadme(readmePath, &info)

		// Scan for agent directories (subdirs with CLAUDE.md)
		if agents := findAgentDirs(filepath.Join(repoDir, e.Name())); len(agents) > 0 {
			info.Agents = strings.Join(agents, ", ")
		}

		if info.Name == "" {
			info.Name = e.Name()
		}

		scaffolds = append(scaffolds, info)
	}

	sort.Slice(scaffolds, func(i, j int) bool {
		return scaffolds[i].Dir < scaffolds[j].Dir
	})
	return scaffolds, nil
}

// parseReadme extracts metadata from a scaffold README.md.
// Supports YAML frontmatter (---delimited) and falls back to parsing the first heading and paragraph.
func parseReadme(path string, info *ScaffoldInfo) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return
	}

	firstLine := scanner.Text()

	// Try frontmatter
	if strings.TrimSpace(firstLine) == "---" {
		fm := parseFrontmatter(scanner)
		if name := fm["name"]; name != "" {
			info.Name = name
		}
		if desc := fm["description"]; desc != "" {
			info.Description = desc
		}
		if hint := fm["install_hint"]; hint != "" {
			info.InstallHint = hint
		}
		return
	}

	// Parse from heading: "# Name — Description"
	if strings.HasPrefix(firstLine, "# ") {
		heading := strings.TrimPrefix(firstLine, "# ")
		if idx := strings.Index(heading, " — "); idx > 0 {
			info.Name = strings.TrimSpace(heading[:idx])
			info.Description = strings.TrimSpace(heading[idx+len(" — "):])
		} else {
			info.Name = heading
		}
	}

	// If no description from heading, use first non-empty paragraph line
	if info.Description == "" {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
				break
			}
			info.Description = line
			break
		}
	}
}

// parseFrontmatter reads key:value pairs from a scanner positioned after the opening ---.
func parseFrontmatter(scanner *bufio.Scanner) map[string]string {
	fm := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			return fm
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			if key != "" {
				fm[key] = val
			}
		}
	}
	return fm
}

// findAgentDirs returns names of subdirectories that contain a CLAUDE.md file.
func findAgentDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var agents []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "CLAUDE.md")); err == nil {
			agents = append(agents, e.Name())
		}
	}
	sort.Strings(agents)
	return agents
}

// EnsureCache clones or updates the templates repo cache.
// Returns the cache directory path.
func EnsureCache() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(home, ".cache", "ttal", "templates")

	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); err == nil {
		cmd := exec.Command("git", "-C", cacheDir, "pull", "--ff-only", "-q")
		if out, pullErr := cmd.CombinedOutput(); pullErr != nil {
			fmt.Fprintf(os.Stderr, "  ! Could not update templates cache (using cached): %s\n",
				strings.TrimSpace(string(out)))
		}
		return cacheDir, nil
	}

	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "clone", "--depth=1", "-q", TemplatesRepo, cacheDir)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone %s: %w", TemplatesRepo, err)
	}
	return cacheDir, nil
}

// copyDir recursively copies a directory tree, skipping dot-directories.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		fi, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, fi.Mode()); err != nil {
			return err
		}
	}

	return nil
}
