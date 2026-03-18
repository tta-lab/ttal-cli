package skill

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const frontmatterDelimiter = "---"

// Skill holds metadata for a single registered skill.
type Skill struct {
	Name        string
	FlicknoteID string
	Category    string
	Description string
}

// skillEntry is the TOML representation of a single skill.
type skillEntry struct {
	ID          string `toml:"id"`
	Category    string `toml:"category"`
	Description string `toml:"description"`
}

// registryFile is the TOML file structure.
type registryFile struct {
	Skills map[string]skillEntry `toml:"skills"`
	Agents map[string][]string   `toml:"agents"`
}

// Registry provides access to the skills registry.
type Registry struct {
	path   string
	data   registryFile
	skills map[string]Skill // keyed by name
}

// Load reads the registry from path. Prints warnings to stderr for dangling
// allow-list references.
func Load(path string) (*Registry, error) {
	r := &Registry{
		path: path,
		data: registryFile{
			Skills: make(map[string]skillEntry),
			Agents: make(map[string][]string),
		},
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Empty registry — not an error
		r.skills = make(map[string]Skill)
		return r, nil
	}

	if _, err := toml.DecodeFile(path, &r.data); err != nil {
		return nil, fmt.Errorf("reading skills registry %s: %w", path, err)
	}

	r.skills = make(map[string]Skill, len(r.data.Skills))
	for name, entry := range r.data.Skills {
		r.skills[name] = Skill{
			Name:        name,
			FlicknoteID: entry.ID,
			Category:    entry.Category,
			Description: entry.Description,
		}
	}

	for _, warning := range r.Validate() {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}

	return r, nil
}

// DefaultPath returns the default path for the skills registry.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "ttal", "skills.toml")
}

// Get returns a skill by name. Returns an error if not found.
func (r *Registry) Get(name string) (*Skill, error) {
	s, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found in registry", name)
	}
	return &s, nil
}

// List returns all skills sorted alphabetically by name.
func (r *Registry) List() []Skill {
	skills := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills
}

// ListForAgent returns skills filtered by the agent's allow-list, sorted alphabetically.
// If the agent is not in the [agents] table, all skills are returned.
func (r *Registry) ListForAgent(agent string) []Skill {
	allowed, ok := r.data.Agents[agent]
	if !ok {
		return r.List()
	}

	skills := make([]Skill, 0, len(allowed))
	for _, name := range allowed {
		if s, ok := r.skills[name]; ok {
			skills = append(skills, s)
		}
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills
}

// Add adds a skill to the registry and saves. If force is false and the name
// already exists, an error is returned.
func (r *Registry) Add(s Skill, force bool) error {
	if _, exists := r.skills[s.Name]; exists && !force {
		return fmt.Errorf("skill %q already registered (use --force to overwrite)", s.Name)
	}

	r.skills[s.Name] = s
	r.data.Skills[s.Name] = skillEntry{
		ID:          s.FlicknoteID,
		Category:    s.Category,
		Description: s.Description,
	}

	return r.save()
}

// Remove removes a skill from the registry and all agent allow-lists, then saves.
// Returns the removed skill and the list of agents it was removed from.
func (r *Registry) Remove(name string) (Skill, []string, error) {
	s, ok := r.skills[name]
	if !ok {
		return Skill{}, nil, fmt.Errorf("skill %q not found in registry", name)
	}

	delete(r.skills, name)
	delete(r.data.Skills, name)

	var clearedFrom []string
	for agent, names := range r.data.Agents {
		filtered := names[:0]
		for _, n := range names {
			if n != name {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) != len(names) {
			r.data.Agents[agent] = filtered
			clearedFrom = append(clearedFrom, agent)
		}
	}
	sort.Strings(clearedFrom)

	return s, clearedFrom, r.save()
}

// Validate returns warnings for dangling allow-list references.
func (r *Registry) Validate() []string {
	var warnings []string
	for agent, names := range r.data.Agents {
		for _, name := range names {
			if _, ok := r.skills[name]; !ok {
				warnings = append(warnings, fmt.Sprintf("agent %q allow-list references unknown skill %q", agent, name))
			}
		}
	}
	sort.Strings(warnings)
	return warnings
}

// ReverseLookup finds a skill by flicknote ID prefix (8-char hex).
func (r *Registry) ReverseLookup(flicknoteID string) (*Skill, bool) {
	for _, s := range r.skills {
		if strings.HasPrefix(s.FlicknoteID, flicknoteID) || s.FlicknoteID == flicknoteID {
			sc := s
			return &sc, true
		}
	}
	return nil, false
}

// save writes the current state back to the TOML file.
func (r *Registry) save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(r.data); err != nil {
		return fmt.Errorf("encoding skills registry: %w", err)
	}

	if err := os.WriteFile(r.path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing skills registry %s: %w", r.path, err)
	}
	return nil
}

// ParseFrontmatter extracts name and description from YAML frontmatter,
// and returns the body content with frontmatter stripped.
// Used by `add --file` and `migrate` to auto-populate skill metadata
// and upload only the body to flicknote (no frontmatter pollution).
func ParseFrontmatter(content []byte) (name, description string, body []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(content))

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != frontmatterDelimiter {
		return "", "", content
	}

	fm := make(map[string]string)
	var afterFM int
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == frontmatterDelimiter {
			found = true
			break
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			val = strings.Trim(val, "\"'")
			fm[key] = val
		}
	}

	if !found {
		return "", "", content
	}

	// Calculate byte offset after the closing ---
	// We need to find where the body starts
	lines := bytes.Split(content, []byte("\n"))
	inFM := false
	closedFM := false
	_ = afterFM
	bodyStart := 0
	consumed := 0

	for _, line := range lines {
		lineLen := len(line) + 1 // +1 for \n
		trimmed := strings.TrimSpace(string(line))

		if !inFM && trimmed == frontmatterDelimiter {
			inFM = true
			consumed += lineLen
			continue
		}
		if inFM && trimmed == frontmatterDelimiter {
			closedFM = true
			consumed += lineLen
			bodyStart = consumed
			break
		}
		if inFM {
			consumed += lineLen
		}
	}

	if !closedFM {
		return "", "", content
	}

	return fm["name"], fm["description"], content[bodyStart:]
}
