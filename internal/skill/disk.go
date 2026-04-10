// Package skill provides skill registry and disk-based skill access.
//
// Plane: shared
package skill

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
)

// ContentFetcher is the function used to fetch a skill's content by name.
// It defaults to fetchContentImpl but can be replaced for testing.
var ContentFetcher func(name string) string = fetchContentImpl

// FetchContent returns the skill content for a named skill, read from disk.
func FetchContent(name string) string { return ContentFetcher(name) }

// fetchContentImpl reads skill content from the default skills disk directory.
// Returns empty string on any error (soft-fail: logs a warning but does not propagate).
func fetchContentImpl(name string) string {
	skill, err := GetSkill(DefaultSkillsDir(), name)
	if err != nil {
		log.Printf("[skill] warning: could not read skill %q from disk: %v", name, err)
		return ""
	}
	return skill.Content
}

// FetchContents calls FetchContent for each name in order and concatenates
// the results, wrapping each skill in a `# <SkillName> [skill]` header.
// Skips empty results silently.
func FetchContents(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var parts []string
	for _, name := range names {
		content := FetchContent(name)
		if content == "" {
			log.Printf("[skill] FetchContents: skill %q not found or empty, skipping", name)
			continue
		}
		parts = append(parts, fmt.Sprintf("# %s [skill]\n\n%s", name, content))
	}
	return strings.Join(parts, "\n\n")
}

// ParseFrontmatter extracts name, description, and category from YAML frontmatter,
// and returns the body content with frontmatter stripped.
// Returns all empty strings when no frontmatter is present (body is returned unchanged).
func ParseFrontmatter(content []byte) (name, description, category string, body []byte) {
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Category    string `yaml:"category"`
	}

	rest, err := frontmatter.Parse(bytes.NewReader(content), &meta)
	if err != nil {
		// No frontmatter or parse error — return content as body
		return "", "", "", content
	}
	return meta.Name, meta.Description, meta.Category, rest
}


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
