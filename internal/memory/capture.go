package memory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/ent"
	"github.com/tta-lab/ttal-cli/ent/project"
)

type Commit struct {
	Hash      string
	Agent     string
	Category  string
	Message   string
	Timestamp time.Time
	Project   string
}

type Capturer struct {
	client *ent.Client
}

func NewCapturer(client *ent.Client) *Capturer {
	return &Capturer{client: client}
}

// Capture scans all projects and generates memory files for a given date
func (c *Capturer) Capture(date time.Time, outputDir string) error {
	ctx := context.Background()

	// Get all active projects with non-empty paths
	projects, err := c.client.Project.Query().
		Where(
			project.ArchivedAtIsNil(),
			project.PathNEQ(""),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("no active projects found")
	}

	// Collect commits from all projects
	var allCommits []Commit
	for _, proj := range projects {
		commits, err := c.extractCommits(proj.Alias, proj.Path, date)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to extract commits from %s: %v\n", proj.Alias, err)
			continue
		}

		allCommits = append(allCommits, commits...)
	}

	if len(allCommits) == 0 {
		return fmt.Errorf("no commits found for date %s", date.Format("2006-01-02"))
	}

	// Group commits by agent
	commitsByAgent := make(map[string][]Commit)
	for _, commit := range allCommits {
		commitsByAgent[commit.Agent] = append(commitsByAgent[commit.Agent], commit)
	}

	// Generate memory file for each agent
	for agent, commits := range commitsByAgent {
		if err := c.writeMemoryFile(agent, date, commits, outputDir); err != nil {
			return fmt.Errorf("failed to write memory file for %s: %w", agent, err)
		}
	}

	fmt.Printf("Memory capture complete: %d agents, %d commits\n", len(commitsByAgent), len(allCommits))
	return nil
}

// extractCommits gets commits from a git repository for a specific date
func (c *Capturer) extractCommits(projectAlias, repoPath string, date time.Time) ([]Commit, error) {
	// Check if path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", repoPath)
	}

	// Format date for git log
	dateStr := date.Format("2006-01-02")

	// Git log command to get commits for a specific date
	// Format: hash|author|timestamp|subject
	cmd := exec.Command("git", "log",
		"--all",
		"--since="+dateStr+" 00:00:00",
		"--until="+dateStr+" 23:59:59",
		"--pretty=format:%H|%an|%at|%s")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	// Parse commits
	lines := strings.Split(string(output), "\n")
	var commits []Commit

	// Regex to parse agent prefix: "agent: [category] message" or "agent: message"
	agentRegex := regexp.MustCompile(`^([a-zA-Z0-9_-]+):\s*(?:\[([^\]]+)\]\s*)?(.+)$`)

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		hash := parts[0]
		author := parts[1]
		timestampStr := parts[2]
		subject := parts[3]

		// Parse timestamp
		var timestamp int64
		_, _ = fmt.Sscanf(timestampStr, "%d", &timestamp)
		commitTime := time.Unix(timestamp, 0)

		// Try to parse agent prefix
		matches := agentRegex.FindStringSubmatch(subject)
		if matches != nil {
			// Found agent prefix
			agent := strings.ToLower(matches[1])
			category := matches[2]
			message := matches[3]

			commits = append(commits, Commit{
				Hash:      hash[:7],
				Agent:     agent,
				Category:  category,
				Message:   message,
				Timestamp: commitTime,
				Project:   projectAlias,
			})
		} else {
			// No agent prefix, use author name as agent
			agent := strings.ToLower(strings.ReplaceAll(author, " ", ""))
			commits = append(commits, Commit{
				Hash:      hash[:7],
				Agent:     agent,
				Category:  "",
				Message:   subject,
				Timestamp: commitTime,
				Project:   projectAlias,
			})
		}
	}

	return commits, nil
}

// writeMemoryFile creates a memory markdown file for an agent
func (c *Capturer) writeMemoryFile(agent string, date time.Time, commits []Commit, outputDir string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create memory file path
	filename := fmt.Sprintf("%s-%s.md", date.Format("2006-01-02"), agent)
	filePath := filepath.Join(outputDir, filename)

	// Group commits by category and project
	type CommitGroup struct {
		Category string
		Project  string
		Commits  []Commit
	}

	groups := make(map[string]*CommitGroup)
	for _, commit := range commits {
		key := fmt.Sprintf("%s|%s", commit.Category, commit.Project)
		if _, ok := groups[key]; !ok {
			groups[key] = &CommitGroup{
				Category: commit.Category,
				Project:  commit.Project,
			}
		}
		groups[key].Commits = append(groups[key].Commits, commit)
	}

	// Generate markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Memory Log: %s - %s\n\n", agent, date.Format("2006-01-02")))
	content.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("Total commits: %d\n\n", len(commits)))

	// Write commits grouped by project and category
	projectGroups := make(map[string][]CommitGroup)
	for _, group := range groups {
		projectGroups[group.Project] = append(projectGroups[group.Project], *group)
	}

	for proj, categoryGroups := range projectGroups {
		content.WriteString(fmt.Sprintf("## Project: %s\n\n", proj))

		for _, group := range categoryGroups {
			if group.Category != "" {
				content.WriteString(fmt.Sprintf("### Category: %s\n\n", group.Category))
			} else {
				content.WriteString("### Uncategorized\n\n")
			}

			for _, commit := range group.Commits {
				content.WriteString(fmt.Sprintf("- `%s` %s (%s)\n",
					commit.Hash,
					commit.Message,
					commit.Timestamp.Format("15:04")))
			}
			content.WriteString("\n")
		}
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Generated: %s\n", filePath)
	return nil
}
