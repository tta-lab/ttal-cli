package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// validStageNamePattern matches stage names that produce valid taskwarrior tags.
var validStageNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// Stage defines a single stage in a pipeline.
type Stage struct {
	Name     string `toml:"name"`
	Assignee string `toml:"assignee"` // role from roles.toml (e.g. "designer") or "coder" (special)
	Gate     string `toml:"gate"`     // "human" or "auto"
	Reviewer string `toml:"reviewer"` // reviewer agent name (e.g. "plan-review-lead"), optional
}

// Pipeline defines a named pipeline with tag filters and stages.
type Pipeline struct {
	Description string   `toml:"description"`
	Tags        []string `toml:"tags"` // task tags that select this pipeline
	Stages      []Stage  `toml:"stages"`
}

// StageTag returns the lowercased stage name used as a taskwarrior tag.
func (s *Stage) StageTag() string {
	return strings.ToLower(s.Name)
}

// StageLGTMTag returns the lgtm tag for this stage (<stagename>_lgtm).
func (s *Stage) StageLGTMTag() string {
	return s.StageTag() + "_lgtm"
}

// LastStage returns the final stage in the pipeline.
func (p *Pipeline) LastStage() *Stage {
	if len(p.Stages) == 0 {
		return nil
	}
	return &p.Stages[len(p.Stages)-1]
}

// StageIndexForRole returns the highest stage index whose Assignee matches the given role.
// Returns -1 if no stage matches.
func (p *Pipeline) StageIndexForRole(role string) int {
	last := -1
	for i, s := range p.Stages {
		if s.Assignee == role {
			last = i
		}
	}
	return last
}

// Config holds all pipeline definitions.
type Config struct {
	Pipelines map[string]Pipeline
}

// SortedNames returns pipeline names in alphabetical order.
func (c *Config) SortedNames() []string {
	names := make([]string, 0, len(c.Pipelines))
	for name := range c.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Summary returns a formatted string listing all pipelines and their tags.
// Used in error messages to help agents pick the right pipeline tag.
func (c *Config) Summary() string {
	if len(c.Pipelines) == 0 {
		return "(no pipelines configured)"
	}

	var lines []string
	for _, name := range c.SortedNames() {
		p := c.Pipelines[name]
		if len(p.Tags) == 0 {
			lines = append(lines, fmt.Sprintf("  %s: (no tags)", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s: +%s", name, strings.Join(p.Tags, ", +")))
	}
	return strings.Join(lines, "\n")
}

// Load reads pipelines.toml from the ttal config directory.
// Uses toml.DecodeFile (NOT toml.Unmarshal which does not exist in BurntSushi/toml v1.x).
// Uses os.IsNotExist for missing file check (same pattern as internal/config/config.go).
func Load(configDir string) (*Config, error) {
	path := filepath.Join(configDir, "pipelines.toml")

	var raw map[string]Pipeline
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		if os.IsNotExist(err) {
			return &Config{Pipelines: make(map[string]Pipeline)}, nil
		}
		return nil, fmt.Errorf("parse pipelines.toml: %w", err)
	}

	cfg := &Config{Pipelines: raw}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// MatchPipeline returns the pipeline name and definition matching the given task tags.
// Returns ("", nil, nil) if no pipeline matches.
// Returns an error if multiple pipelines match (tag overlap).
func (c *Config) MatchPipeline(taskTags []string) (string, *Pipeline, error) {
	tagSet := make(map[string]bool, len(taskTags))
	for _, t := range taskTags {
		tagSet[t] = true
	}

	var matches []string
	for name, p := range c.Pipelines {
		for _, ft := range p.Tags {
			if tagSet[ft] {
				matches = append(matches, name)
				break
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", nil, nil
	case 1:
		p := c.Pipelines[matches[0]]
		return matches[0], &p, nil
	default:
		return "", nil, fmt.Errorf("task tags match multiple pipelines: %v", matches)
	}
}

// ReviewerForStage returns the Reviewer field of the first stage whose Assignee
// matches assigneeRole in the pipeline matching taskTags.
// Returns "" if no pipeline matches or no stage has the given assignee.
func (c *Config) ReviewerForStage(taskTags []string, assigneeRole string) string {
	_, p, err := c.MatchPipeline(taskTags)
	if err != nil || p == nil {
		return ""
	}
	for _, s := range p.Stages {
		if s.Assignee == assigneeRole {
			return s.Reviewer
		}
	}
	return ""
}

// NotifyTarget represents the notification counterpart for a reviewer agent.
type NotifyTarget int

const (
	NotifyTargetNone     NotifyTarget = iota
	NotifyTargetCoder                 // reviewer reviews coder stages → notify coder
	NotifyTargetDesigner              // reviewer reviews non-coder stages → notify designer
)

// ReviewerNotifyTarget scans all pipelines to determine what notification
// target an agent maps to based on the stage type they review.
// Returns NotifyTargetCoder if the agent is a reviewer for a "coder" stage.
// Returns NotifyTargetDesigner if the agent is a reviewer for any other stage.
// Returns NotifyTargetNone if the agent is not a reviewer in any pipeline.
// If the agent reviews both coder and non-coder stages, NotifyTargetCoder wins.
func (c *Config) ReviewerNotifyTarget(agentName string) NotifyTarget {
	target := NotifyTargetNone
	for _, p := range c.Pipelines {
		for _, s := range p.Stages {
			if s.Reviewer != agentName {
				continue
			}
			if s.Assignee == "coder" {
				return NotifyTargetCoder // most specific — return immediately
			}
			target = NotifyTargetDesigner
		}
	}
	return target
}

// HasReviewer returns true if agentName is configured as a reviewer in any pipeline stage.
func (c *Config) HasReviewer(agentName string) bool {
	for _, p := range c.Pipelines {
		for _, s := range p.Stages {
			if s.Reviewer == agentName {
				return true
			}
		}
	}
	return false
}

// CurrentStage determines which pipeline stage is currently active by looking
// for monotonic stage tags. Each stage adds +<stagename> on entry and
// +<stagename>_lgtm on reviewer approval. The active stage is the latest
// stage tag without a corresponding _lgtm tag.
//
// taskTags is the list of tags on the task.
//
// Returns (stageIndex, *Stage, nil) if a stage is found.
// Returns (-1, nil, nil) if no stage tag matches — task not started.
func (p *Pipeline) CurrentStage(taskTags []string) (int, *Stage, error) {
	tagSet := make(map[string]bool, len(taskTags))
	for _, t := range taskTags {
		tagSet[t] = true
	}

	lastEnteredIdx := -1
	var lastEnteredStage *Stage

	for i := range p.Stages {
		stageTag := p.Stages[i].StageTag()
		if !tagSet[stageTag] {
			continue
		}
		lastEnteredIdx = i
		lastEnteredStage = &p.Stages[i]
		if !tagSet[p.Stages[i].StageLGTMTag()] {
			return i, &p.Stages[i], nil
		}
	}

	if lastEnteredIdx >= 0 {
		return lastEnteredIdx, lastEnteredStage, nil
	}

	return -1, nil, nil
}

func (c *Config) validate() error {
	for name := range c.Pipelines {
		p := c.Pipelines[name]
		if len(p.Stages) == 0 {
			return fmt.Errorf("pipeline %q has no stages", name)
		}
		if len(p.Tags) == 0 {
			return fmt.Errorf("pipeline %q has no tag filters", name)
		}
		for i := range p.Stages {
			s := &p.Stages[i]
			if s.Name == "" {
				return fmt.Errorf("pipeline %q stage %d has no name", name, i)
			}
			if s.Assignee == "" {
				return fmt.Errorf("pipeline %q stage %q has no assignee", name, s.Name)
			}
			if s.Gate != "human" && s.Gate != "auto" {
				return fmt.Errorf("pipeline %q stage %q: gate must be \"human\" or \"auto\", got %q", name, s.Name, s.Gate)
			}
			// Stage names must be valid taskwarrior tags (alphanumeric + underscore only).
			if !validStageNamePattern.MatchString(s.Name) {
				return fmt.Errorf(
					"pipeline %q stage %q: name must be alphanumeric (a-z, A-Z, 0-9, _) — no spaces or hyphens",
					name, s.Name,
				)
			}
		}
		c.Pipelines[name] = p // write back — Go maps return value copies
	}

	// Check for overlapping tag filters across all pipelines
	tagOwner := make(map[string]string)
	for name, p := range c.Pipelines {
		for _, tag := range p.Tags {
			if owner, ok := tagOwner[tag]; ok {
				return fmt.Errorf("tag %q claimed by both pipeline %q and %q", tag, owner, name)
			}
			tagOwner[tag] = name
		}
	}

	return nil
}
