package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorkerAgentResult tracks a single worker agent deployment for reporting.
type WorkerAgentResult struct {
	Source string
	Name   string
	Dest   string
}

// DeployWorkerAgents scans each path in workerAgentPaths for .md agent files,
// converts them to CC-native format via GenerateCCVariant, and writes to
// ~/.claude/agents/{name}.md. Files with managed_by: ttal-sync are tracked as
// managed by this process.
func DeployWorkerAgents(workerAgentPaths []string, dryRun bool) ([]WorkerAgentResult, error) {
	if len(workerAgentPaths) == 0 {
		return nil, nil
	}

	agentsDir := claudeAgentsDir()
	if !dryRun {
		if err := os.MkdirAll(agentsDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating agents dir %s: %w", agentsDir, err)
		}
	}

	var results []WorkerAgentResult

	for _, srcDir := range workerAgentPaths {
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			return nil, fmt.Errorf("read worker agent dir %s: %w", srcDir, err)
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if isSkipFile(e.Name()) {
				continue
			}

			srcPath := filepath.Join(srcDir, e.Name())
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", srcPath, err)
			}

			parsed, err := ParseAgentFile(string(data))
			if err != nil {
				return nil, fmt.Errorf("parse agent file %s: %w", srcPath, err)
			}

			ccVariant, err := GenerateCCVariant(parsed)
			if err != nil {
				return nil, fmt.Errorf("generate CC variant for %s: %w", parsed.Frontmatter.Name, err)
			}

			dstPath := filepath.Join(agentsDir, parsed.Frontmatter.Name+".md")
			if !dryRun {
				if err := os.WriteFile(dstPath, []byte(ccVariant), 0o644); err != nil {
					return nil, fmt.Errorf("write %s: %w", dstPath, err)
				}
			}

			results = append(results, WorkerAgentResult{
				Source: srcPath,
				Name:   parsed.Frontmatter.Name,
				Dest:   dstPath,
			})
		}
	}

	return results, nil
}

// DeployManagerAgents scans teamPath for per-agent workspace subdirectories
// (yuki/, kestrel/, sage/, ...) containing an AGENTS.md identity file,
// converts each via GenerateCCVariant, and writes to ~/.claude/agents/{name}.md.
// Top-level files (CLAUDE.user.md, README.md) and dirs without AGENTS.md are skipped.
func DeployManagerAgents(teamPath string, dryRun bool) ([]WorkerAgentResult, error) {
	if teamPath == "" {
		return nil, nil
	}

	agentsDir := claudeAgentsDir()
	if !dryRun {
		if err := os.MkdirAll(agentsDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating agents dir %s: %w", agentsDir, err)
		}
	}

	entries, err := os.ReadDir(teamPath)
	if err != nil {
		return nil, fmt.Errorf("read team path %s: %w", teamPath, err)
	}

	var results []WorkerAgentResult
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		srcPath := filepath.Join(teamPath, name, "AGENTS.md")
		data, err := os.ReadFile(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // workspace dir without an identity file — skip
			}
			return nil, fmt.Errorf("read %s: %w", srcPath, err)
		}

		parsed, err := ParseAgentFile(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse agent file %s: %w", srcPath, err)
		}

		ccVariant, err := GenerateCCVariant(parsed)
		if err != nil {
			return nil, fmt.Errorf("generate CC variant for %s: %w", parsed.Frontmatter.Name, err)
		}

		dstPath := filepath.Join(agentsDir, parsed.Frontmatter.Name+".md")
		if !dryRun {
			if err := os.WriteFile(dstPath, []byte(ccVariant), 0o644); err != nil {
				return nil, fmt.Errorf("write %s: %w", dstPath, err)
			}
		}

		results = append(results, WorkerAgentResult{
			Source: srcPath,
			Name:   parsed.Frontmatter.Name,
			Dest:   dstPath,
		})
	}

	return results, nil
}

// claudeAgentsDir returns the path to ~/.claude/agents/.
func claudeAgentsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "agents")
}

// isSkipFile returns true for known non-agent files to exclude from sync.
func isSkipFile(name string) bool {
	switch name {
	case "CLAUDE.md", "README.md", "CLAUDE.user.md":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}
