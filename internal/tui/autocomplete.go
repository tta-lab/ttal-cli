package tui

import (
	"log"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const (
	modifierProject = "project:"
	modifierTag     = "+"
)

const (
	matchTypeProject = "project"
	matchTypeTag     = "tag"
)

type modifyMatch struct {
	Type  string
	Value string
}

func (m *Model) updateProjectMatches(projects []string, query string) {
	q := strings.ToLower(query)
	for _, p := range projects {
		if q == "" || strings.Contains(strings.ToLower(p), q) {
			m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeProject, Value: p})
		}
	}
}

func (m *Model) updateTagMatches(tags []string, query string) {
	q := strings.ToLower(query)
	for _, t := range tags {
		if q == "" || strings.Contains(strings.ToLower(t), q) {
			m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeTag, Value: t})
		}
	}
}

func (m *Model) updateAllMatches(projects, tags []string) {
	for _, p := range projects {
		m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeProject, Value: p})
	}
	for _, t := range tags {
		m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeTag, Value: t})
	}
}

func (m *Model) updateModifyMatches(projects, tags []string) {
	if len(projects) == 0 || len(tags) == 0 {
		var err error
		projects, err = taskwarrior.GetProjects()
		if err != nil {
			log.Printf("failed to get projects: %v", err)
		}
		tags, err = taskwarrior.GetTags()
		if err != nil {
			log.Printf("failed to get tags: %v", err)
		}
	}
	m.updateMatchesWithInput(m.modifyInput, projects, tags)
}

func (m *Model) updateMatchesWithInput(input string, projects, tags []string) {
	m.modifyMatches = nil

	switch {
	case strings.Contains(input, ":"):
		parts := strings.SplitN(input, ":", 2)
		prefix := parts[0]
		value := ""
		if len(parts) > 1 {
			value = strings.TrimSpace(parts[1])
		}
		if prefix == "project" {
			m.updateProjectMatches(projects, value)
		}
	case strings.HasPrefix(input, modifierTag):
		m.updateTagMatches(tags, strings.TrimPrefix(input, modifierTag))
	case input == "":
		m.updateAllMatches(projects, tags)
	default:
		m.updateProjectMatches(projects, input)
		m.updateTagMatches(tags, input)
	}
}

func (m *Model) updateSearchMatches(projects, tags []string) {
	if len(projects) == 0 || len(tags) == 0 {
		var err error
		projects, err = taskwarrior.GetProjects()
		if err != nil {
			log.Printf("failed to get projects: %v", err)
		}
		tags, err = taskwarrior.GetTags()
		if err != nil {
			log.Printf("failed to get tags: %v", err)
		}
	}
	m.updateSearchMatchesWithInput(m.searchStr, projects, tags)
}

func (m *Model) updateSearchMatchesWithInput(input string, projects, tags []string) {
	m.updateMatchesWithInput(input, projects, tags)
}

func deleteLastWord(s string) string {
	s = strings.TrimRight(s, " ")
	if s == "" {
		return ""
	}
	lastSpace := strings.LastIndex(s, " ")
	if lastSpace == -1 {
		return ""
	}
	return s[:lastSpace]
}
