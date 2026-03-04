package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// RuleResult records one RULE.md deployment.
type RuleResult struct {
	Source string // Full path to RULE.md
	Name   string // Rule name (directory name)
	Dest   string // ~/.claude/rules/{name}.md
}

// DeployRules scans rules_paths for directories containing RULE.md
// and copies them to ~/.claude/rules/{name}.md.
func DeployRules(rulesPaths []string, dryRun bool) ([]RuleResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	rulesDir := filepath.Join(home, ".claude", "rules")
	return DeployRulesTo(rulesPaths, rulesDir, dryRun)
}

// DeployRulesTo is like DeployRules but writes to a custom destination directory.
// Used by tests to avoid touching ~/.claude/rules/.
func DeployRulesTo(rulesPaths []string, rulesDir string, dryRun bool) ([]RuleResult, error) {
	var results []RuleResult
	for _, raw := range rulesPaths {
		dir := config.ExpandHome(raw)
		rs, err := deployRulesFromDir(dir, rulesDir, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: rules path %s: %v\n", raw, err)
			continue
		}
		results = append(results, rs...)
	}
	return results, nil
}

func deployRulesFromDir(sourceDir, rulesDir string, dryRun bool) ([]RuleResult, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}

	results := make([]RuleResult, 0, len(entries))

	// Collect all rule sources: subdirectories with RULE.md + direct RULE.md at root.
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rulePath := filepath.Join(sourceDir, e.Name(), "RULE.md")
		if _, err := os.Stat(rulePath); err != nil {
			continue
		}
		dest := filepath.Join(rulesDir, e.Name()+".md")
		results = append(results, RuleResult{Source: rulePath, Name: e.Name(), Dest: dest})
	}

	// Check if sourceDir itself contains RULE.md (e.g. rules_paths entry is a project dir directly).
	directRule := filepath.Join(sourceDir, "RULE.md")
	if _, err := os.Stat(directRule); err == nil {
		name := filepath.Base(sourceDir)
		dest := filepath.Join(rulesDir, name+".md")
		results = append(results, RuleResult{Source: directRule, Name: name, Dest: dest})
	}

	if dryRun || len(results) == 0 {
		return results, nil
	}

	// Create destination directory once before writing.
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return results, err
	}

	for _, r := range results {
		content, err := os.ReadFile(r.Source)
		if err != nil {
			log.Printf("[sync] warning: failed to read %s: %v", r.Source, err)
			continue
		}
		if err := os.WriteFile(r.Dest, content, 0o644); err != nil {
			log.Printf("[sync] warning: failed to write rule %s: %v", r.Dest, err)
		}
	}

	return results, nil
}

// CleanRules removes rule files from a rules directory that don't correspond
// to any RULE.md found in the configured rules_paths.
func CleanRules(rulesPaths []string, dryRun bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	rulesDir := filepath.Join(home, ".claude", "rules")
	return CleanRulesIn(rulesPaths, rulesDir, dryRun)
}

// CleanRulesIn removes rule files from rulesDir that don't correspond
// to any RULE.md found in the configured rules_paths.
func CleanRulesIn(rulesPaths []string, rulesDir string, dryRun bool) ([]string, error) {
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Build set of valid rule names from current deployment.
	current, _ := DeployRulesTo(rulesPaths, rulesDir, true) // dry-run to discover
	valid := make(map[string]bool)
	for _, r := range current {
		valid[r.Name] = true
	}

	removed := make([]string, 0, len(entries))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		if name == e.Name() {
			continue // not a .md file
		}
		if valid[name] {
			continue
		}
		path := filepath.Join(rulesDir, e.Name())
		removed = append(removed, path)
		if !dryRun {
			os.Remove(path)
		}
	}
	return removed, nil
}

const (
	codexRulesMarkerStart = "<!-- ttal-rules-start -->"
	codexRulesMarkerEnd   = "<!-- ttal-rules-end -->"
)

// DeployCodexRules aggregates all RULE.md contents into ~/.codex/AGENTS.md
// under a managed section delimited by HTML comment markers.
func DeployCodexRules(rules []RuleResult, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codexDir := filepath.Join(home, ".codex")
	if _, err := os.Stat(codexDir); os.IsNotExist(err) {
		return nil // Codex not installed
	}

	agentsPath := filepath.Join(codexDir, "AGENTS.md")
	return DeployCodexRulesTo(rules, agentsPath, dryRun)
}

// DeployCodexRulesTo writes aggregated rules to a specific AGENTS.md path.
// Used by tests to avoid touching ~/.codex/AGENTS.md.
func DeployCodexRulesTo(rules []RuleResult, agentsPath string, dryRun bool) error {
	existing, _ := os.ReadFile(agentsPath)
	content := string(existing)

	// Build managed section.
	var sb strings.Builder
	sb.WriteString(codexRulesMarkerStart + "\n")
	sb.WriteString("## Shared Knowledge\n\n")
	for _, r := range rules {
		ruleContent, err := os.ReadFile(r.Source)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", r.Name))
		sb.WriteString(strings.TrimSpace(string(ruleContent)))
		sb.WriteString("\n\n")
	}
	sb.WriteString(codexRulesMarkerEnd)

	// Replace or append.
	startIdx := strings.Index(content, codexRulesMarkerStart)
	endIdx := strings.Index(content, codexRulesMarkerEnd)
	if startIdx >= 0 && endIdx >= 0 {
		content = content[:startIdx] + sb.String() + content[endIdx+len(codexRulesMarkerEnd):]
	} else {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + sb.String() + "\n"
	}

	if dryRun {
		return nil
	}
	return os.WriteFile(agentsPath, []byte(content), 0o644)
}
