package agentfs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AgentInfo holds agent metadata parsed from CLAUDE.md frontmatter.
type AgentInfo struct {
	Name        string // directory name (lowercase)
	Path        string // absolute path to agent directory
	Voice       string // Kokoro TTS voice ID
	Emoji       string // display emoji
	Description string // short role summary
}

// Discover scans teamPath for agent directories (subdirs with CLAUDE.md).
// Returns sorted list of agents with metadata parsed from frontmatter.
func Discover(teamPath string) ([]AgentInfo, error) {
	entries, err := os.ReadDir(teamPath)
	if err != nil {
		return nil, fmt.Errorf("read team path %s: %w", teamPath, err)
	}

	var agents []AgentInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		claudeMd := filepath.Join(teamPath, e.Name(), "CLAUDE.md")
		if _, err := os.Stat(claudeMd); err != nil {
			continue
		}

		info := AgentInfo{
			Name: e.Name(),
			Path: filepath.Join(teamPath, e.Name()),
		}

		if fm, err := parseFrontmatter(claudeMd); err == nil {
			info.Voice = fm["voice"]
			info.Emoji = fm["emoji"]
			info.Description = fm["description"]
		}

		agents = append(agents, info)
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents, nil
}

// Get returns metadata for a single agent by name.
func Get(teamPath, name string) (*AgentInfo, error) {
	agentDir := filepath.Join(teamPath, name)
	claudeMd := filepath.Join(agentDir, "CLAUDE.md")

	if _, err := os.Stat(claudeMd); err != nil {
		return nil, fmt.Errorf("agent '%s' not found (no CLAUDE.md in %s)", name, agentDir)
	}

	info := &AgentInfo{
		Name: name,
		Path: agentDir,
	}

	if fm, err := parseFrontmatter(claudeMd); err == nil {
		info.Voice = fm["voice"]
		info.Emoji = fm["emoji"]
		info.Description = fm["description"]
	}

	return info, nil
}

// Count returns the number of agent directories in teamPath.
func Count(teamPath string) (int, error) {
	agents, err := Discover(teamPath)
	if err != nil {
		return 0, err
	}
	return len(agents), nil
}

// SetField updates a single frontmatter field in an agent's CLAUDE.md.
// If no frontmatter exists, it adds one. Preserves existing content.
func SetField(teamPath, name, field, value string) error {
	claudeMd := filepath.Join(teamPath, name, "CLAUDE.md")
	data, err := os.ReadFile(claudeMd)
	if err != nil {
		return fmt.Errorf("read %s: %w", claudeMd, err)
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
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, fm[k]))
		}
	}
	sb.WriteString("---\n")
	sb.WriteString(body)

	return os.WriteFile(claudeMd, []byte(sb.String()), 0o644)
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

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return nil, fmt.Errorf("no frontmatter")
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
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
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, content
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
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
