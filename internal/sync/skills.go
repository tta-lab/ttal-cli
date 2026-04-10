// Package sync deploys skills from source directories to ~/.agents/skills/.
//
// Plane: manager
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillResult records a single skill deployment.
type SkillResult struct {
	Source string // absolute path to source SKILL.md
	Dest   string // absolute path to destination file
	Name   string // skill name (directory name or stem)
}

// DeploySkills copies SKILL.md files from skillsPaths source directories
// to the destDir. Files are copied as-is (frontmatter preserved).
// For dir-based skills ({name}/SKILL.md), destination is {destDir}/{name}.md.
// For flat files ({name}.md), destination is {destDir}/{name}.md.
func DeploySkills(skillsPaths []string, destDir string, dryRun bool) ([]SkillResult, error) {
	var results []SkillResult

	if err := os.MkdirAll(destDir, 0o755); err != nil && !dryRun {
		return nil, fmt.Errorf("creating skills dest dir: %w", err)
	}

	for _, basePath := range skillsPaths {
		src, err := filepath.EvalSymlinks(basePath)
		if err != nil {
			return nil, fmt.Errorf("resolving skills path %q: %w", basePath, err)
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return nil, fmt.Errorf("reading skills directory %q: %w", src, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".md") {
				continue // skip non-skill files
			}

			var srcFile, destFile string
			var skillName string

			if entry.IsDir() {
				// Dir-based: {name}/SKILL.md → {destDir}/{name}.md
				skillName = entry.Name()
				srcFile = filepath.Join(src, entry.Name(), "SKILL.md")
				destFile = filepath.Join(destDir, skillName+".md")
			} else {
				// Flat file: {name}.md → {destDir}/{name}.md
				stem := strings.TrimSuffix(entry.Name(), ".md")
				skillName = stem
				srcFile = filepath.Join(src, entry.Name())
				destFile = filepath.Join(destDir, stem+".md")
			}

			if _, err := os.Stat(srcFile); os.IsNotExist(err) {
				continue // no SKILL.md in this dir, skip
			}

			results = append(results, SkillResult{
				Source: srcFile,
				Dest:   destFile,
				Name:   skillName,
			})

			if !dryRun {
				if err := copyFile(srcFile, destFile); err != nil {
					return nil, fmt.Errorf("copying %s: %w", srcFile, err)
				}
			}
		}
	}

	return results, nil
}
