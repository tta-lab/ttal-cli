package skill

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// DefaultPath is the default path for the skills registry.
// It is a variable (not a function) so tests can patch it.
var DefaultPath = func() string {
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
		if strings.HasPrefix(s.FlicknoteID, flicknoteID) {
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

// FetchContent loads the default skills registry, looks up the skill by name,
// and returns its raw flicknote content. Returns empty string on any error
// (soft-fail: logs a warning but does not propagate the error).
func FetchContent(name string) string {
	r, err := Load(DefaultPath())
	if err != nil {
		log.Printf("[skill] warning: could not load skills registry: %v", err)
		return ""
	}
	s, err := r.Get(name)
	if err != nil {
		log.Printf("[skill] warning: skill %q not found in registry: %v", name, err)
		return ""
	}

	content, err := FlicknoteFetcher(s.FlicknoteID)
	if err != nil {
		log.Printf("[skill] warning: could not fetch flicknote content for %q: %v", name, err)
		return ""
	}
	// Normalize: strip trailing newline so callers get consistent content without trailing \n.
	return strings.TrimRight(content, "\n")
}

// FlicknoteFetcher is the function used to fetch skill content from flicknote.
// It defaults to fetchFlicknoteContent but can be replaced for testing.
var FlicknoteFetcher func(id string) (string, error) = fetchFlicknoteContent

// fetchFlicknoteContent shells out to `flicknote content <id> --raw` and returns
// the stdout content.
func fetchFlicknoteContent(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "flicknote", "content", id, "--raw")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("flicknote content %s: %w", id, err)
	}
	return strings.TrimSuffix(stdout.String(), "\n"), nil
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
			continue
		}
		parts = append(parts, fmt.Sprintf("# %s [skill]\n\n%s", name, content))
	}
	return strings.Join(parts, "\n\n")
}

// ParseFrontmatter extracts name and description from YAML frontmatter,
// and returns the body content with frontmatter stripped.
// Used by `add --file` and `migrate` to auto-populate skill metadata
// and upload only the body to flicknote (no frontmatter pollution).
//
// Single-pass over bytes.Split lines so bodyStart tracks real byte offsets.
// Handles both LF and CRLF line endings correctly.
func ParseFrontmatter(content []byte) (name, description string, body []byte) {
	lines := bytes.Split(content, []byte("\n"))

	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != frontmatterDelimiter {
		return "", "", content
	}

	fm := make(map[string]string)
	consumed := len(lines[0]) + 1 // opening --- line + \n

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		lineLen := len(line) + 1 // +1 for the \n separator (handles CRLF: \r stays in len)
		trimmed := strings.TrimSpace(string(line))

		if trimmed == frontmatterDelimiter {
			consumed += lineLen
			if consumed > len(content) {
				return fm["name"], fm["description"], []byte{}
			}
			return fm["name"], fm["description"], content[consumed:]
		}

		if idx := bytes.IndexByte(line, ':'); idx > 0 {
			key := strings.TrimSpace(string(line[:idx]))
			val := strings.TrimSpace(string(line[idx+1:]))
			val = strings.Trim(val, "\"'")
			fm[key] = val
		}
		consumed += lineLen
	}

	return "", "", content // unterminated frontmatter
}
