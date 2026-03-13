package agentfs

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const frontmatterDelimiter = "---"

// SkipFiles contains known non-agent files to exclude from discovery.
var SkipFiles = map[string]bool{
	"CLAUDE":      true,
	"CLAUDE.user": true,
	"README":      true,
}

// isSkipFile returns true if the filename should be excluded from agent discovery.
func isSkipFile(name string) bool {
	return SkipFiles[name]
}

// isAgentFile returns true if the entry is a valid agent .md file.
func isAgentFile(e fs.DirEntry) (name string, ok bool) {
	if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
		return "", false
	}
	if !strings.HasSuffix(e.Name(), ".md") {
		return "", false
	}
	name = strings.TrimSuffix(e.Name(), ".md")
	if isSkipFile(name) {
		return "", false
	}
	return name, true
}

// AgentInfo holds agent metadata parsed from .md file frontmatter.
type AgentInfo struct {
	Name             string // directory name (lowercase)
	Path             string // absolute path to agent directory
	Voice            string // Kokoro TTS voice ID
	Emoji            string // display emoji
	Description      string // short role summary
	Role             string // e.g. designer, researcher — matches [prompts] key
	FlicknoteProject string // default flicknote project for this agent
}

// Discover scans teamPath for agents via flat .md files (e.g., yuki.md).
// Returns sorted list of agents with metadata parsed from frontmatter.
func Discover(teamPath string) ([]AgentInfo, error) {
	entries, err := os.ReadDir(teamPath)
	if err != nil {
		return nil, fmt.Errorf("read team path %s: %w", teamPath, err)
	}

	agents := make([]AgentInfo, 0, len(entries))

	for _, e := range entries {
		name, ok := isAgentFile(e)
		if !ok {
			continue
		}

		mdPath := filepath.Join(teamPath, e.Name())

		info := AgentInfo{
			Name: name,
			Path: teamPath,
		}

		if fm, err := parseFrontmatter(mdPath); err == nil {
			info.Voice = fm["voice"]
			info.Emoji = fm["emoji"]
			info.Description = fm["description"]
			info.Role = fm["role"]
			info.FlicknoteProject = fm["flicknote_project"]
		} else {
			log.Printf("agentfs: failed to parse frontmatter for %s: %v", name, err)
		}

		agents = append(agents, info)
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents, nil
}

// Get returns metadata for a single agent by name.
// Looks for name.md in team root.
func Get(teamPath, name string) (*AgentInfo, error) {
	mdPath := filepath.Join(teamPath, name+".md")

	if _, err := os.Stat(mdPath); err != nil {
		return nil, fmt.Errorf("agent '%s' not found (no %s.md in %s)", name, name, teamPath)
	}

	info := &AgentInfo{
		Name: name,
		Path: teamPath,
	}

	if fm, err := parseFrontmatter(mdPath); err == nil {
		info.Voice = fm["voice"]
		info.Emoji = fm["emoji"]
		info.Description = fm["description"]
		info.Role = fm["role"]
		info.FlicknoteProject = fm["flicknote_project"]
	} else {
		log.Printf("agentfs: failed to parse frontmatter for %s: %v", name, err)
	}

	return info, nil
}

// GetFromPath returns agent metadata from an absolute agent directory path.
func GetFromPath(agentPath string) (*AgentInfo, error) {
	return Get(filepath.Dir(agentPath), filepath.Base(agentPath))
}

// DiscoverAgents returns sorted agent names from flat .md files in team root.
func DiscoverAgents(teamPath string) ([]string, error) {
	entries, err := os.ReadDir(teamPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read team path %s: %w", teamPath, err)
	}

	var agents []string
	for _, e := range entries {
		name, ok := isAgentFile(e)
		if !ok {
			continue
		}
		agents = append(agents, name)
	}
	sort.Strings(agents)
	return agents, nil
}

// HasAgent returns true if an agent with the given name exists in teamPath.
func HasAgent(teamPath, agentName string) bool {
	agentFile := filepath.Join(teamPath, agentName+".md")
	_, err := os.Stat(agentFile)
	return err == nil
}

// FindByRole returns all agents with a matching role field.
func FindByRole(teamPath, role string) ([]AgentInfo, error) {
	agents, err := Discover(teamPath)
	if err != nil {
		return nil, err
	}
	var matches []AgentInfo
	for _, a := range agents {
		if a.Role == role {
			matches = append(matches, a)
		}
	}
	return matches, nil
}

// Count returns the number of agent directories in teamPath.
func Count(teamPath string) (int, error) {
	agents, err := Discover(teamPath)
	if err != nil {
		return 0, err
	}
	return len(agents), nil
}

// SetField updates a single frontmatter field in an agent's .md file.
// If no frontmatter exists, it adds one. Preserves existing content.
func SetField(teamPath, name, field, value string) error {
	mdPath := filepath.Join(teamPath, name+".md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", mdPath, err)
	}

	content := string(data)
	fm, body := splitFrontmatter(content)

	fm[field] = value

	var sb strings.Builder
	sb.WriteString("---\n")
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if fm[k] != "" {
			fmt.Fprintf(&sb, "%s: %s\n", k, fm[k])
		}
	}
	sb.WriteString("---\n")
	sb.WriteString(body)

	return os.WriteFile(mdPath, []byte(sb.String()), 0o644)
}

// parseFrontmatter reads YAML-like frontmatter from a markdown file.
func parseFrontmatter(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	fm := make(map[string]string)

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != frontmatterDelimiter {
		return nil, fmt.Errorf("no frontmatter")
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == frontmatterDelimiter {
			return fm, nil
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			val = strings.Trim(val, "\"'")
			fm[key] = val
		}
	}

	return nil, fmt.Errorf("unterminated frontmatter")
}

// splitFrontmatter separates a markdown file into frontmatter map and body string.
func splitFrontmatter(content string) (map[string]string, string) {
	fm := make(map[string]string)

	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontmatterDelimiter {
		return fm, content
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == frontmatterDelimiter {
			for _, line := range lines[1:i] {
				if idx := strings.Index(line, ":"); idx > 0 {
					key := strings.TrimSpace(line[:idx])
					val := strings.TrimSpace(line[idx+1:])
					val = strings.Trim(val, "\"'")
					fm[key] = val
				}
			}
			body := strings.Join(lines[i+1:], "\n")
			return fm, body
		}
	}

	return fm, content
}
