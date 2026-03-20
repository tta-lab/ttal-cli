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
	Assignee string `toml:"assignee"` // role from roles.toml (e.g. "designer") or "worker" (special)
	Gate     string `toml:"gate"`     // "human" or "auto"
	Reviewer string `toml:"reviewer"` // subagent name (e.g. "plan-reviewer"), optional
	Mode     string `toml:"mode"`     // "subagent" or "tmux", defaults to "subagent"
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
			if s.Mode == "" {
				p.Stages[i].Mode = "subagent"
			} else if s.Mode != "subagent" && s.Mode != "tmux" {
				return fmt.Errorf("pipeline %q stage %q: mode must be \"subagent\" or \"tmux\", got %q", name, s.Name, s.Mode)
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
