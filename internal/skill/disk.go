// Package skill provides skill registry and disk-based skill access.
//
// Plane: shared
package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// DefaultSkillsDir returns the default path for deployed skills.
func DefaultSkillsDir() string {
	if dir := os.Getenv("TTAL_SKILLS_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".agents", "skills")
	}
	return filepath.Join(home, ".agents", "skills")
}

// DiskSkill describes a skill on disk.
type DiskSkill struct {
	Name        string
	Description string
	Category    string
	Content     string // body with frontmatter stripped
}

// ListSkills scans dir for *.md files and returns parsed skills.
// Files are scanned for YAML frontmatter; if present, name and description
// are extracted. The body (with frontmatter stripped) is also returned.
func ListSkills(dir string) ([]DiskSkill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []DiskSkill
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		content, err := os.ReadFile(filepath.Join(dir, name+".md"))
		if err != nil {
			continue
		}
		skillName, description, category, body := ParseFrontmatter(content)
		if skillName != "" {
			name = skillName
		}
		skills = append(skills, DiskSkill{
			Name:        name,
			Description: description,
			Category:    category,
			Content:     string(body),
		})
	}
	return skills, nil
}

// GetSkill reads a skill file from disk. Returns error if not found.
func GetSkill(dir, name string) (*DiskSkill, error) {
	path := filepath.Join(dir, name+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	skillName, description, category, body := ParseFrontmatter(content)
	if skillName != "" {
		name = skillName
	}
	return &DiskSkill{
		Name:        name,
		Description: description,
		Category:    category,
		Content:     string(bytes.TrimSpace(body)),
	}, nil
}
