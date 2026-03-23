package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

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

// Config holds all pipeline definitions.
type Config struct {
	Pipelines map[string]Pipeline
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
	NotifyTargetDesigner              // reviewer reviews non-worker stages → notify designer
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

// CurrentStage determines which pipeline stage is currently active by finding
// which agent name tag is present on the task and mapping it to a stage via role.
//
// agentRoles maps agent names to their roles (e.g. {"inke": "designer", "athena": "researcher"}).
// taskTags is the list of tags on the task.
//
// Returns (stageIndex, *Stage, nil) if a stage is found.
// Returns (-1, nil, nil) if no agent tag matches any stage — task not started.
func (p *Pipeline) CurrentStage(taskTags []string, agentRoles map[string]string) (int, *Stage, error) {
	tagSet := make(map[string]bool, len(taskTags))
	for _, t := range taskTags {
		tagSet[t] = true
	}

	// Collect all matching stages to detect ambiguity.
	type match struct {
		idx   int
		stage *Stage
		agent string
	}
	var matches []match
	for _, agentName := range taskTags {
		role, ok := agentRoles[agentName]
		if !ok {
			continue
		}
		for i := range p.Stages {
			if p.Stages[i].Assignee == role {
				matches = append(matches, match{i, &p.Stages[i], agentName})
				break
			}
		}
	}

	// Check for +coder tag (coder stages don't have agent names in agentRoles).
	if tagSet["coder"] {
		for i := range p.Stages {
			if p.Stages[i].Assignee == "coder" {
				matches = append(matches, match{i, &p.Stages[i], "coder"})
				break
			}
		}
	}

	if len(matches) > 1 {
		agents := make([]string, len(matches))
		for i, m := range matches {
			agents[i] = m.agent
		}
		return -1, nil, fmt.Errorf("ambiguous stage: multiple agent tags found %v — remove extra tags", agents)
	}
	if len(matches) == 1 {
		return matches[0].idx, matches[0].stage, nil
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
