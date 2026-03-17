package tui

import (
	"log"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/flicktask"
)

var (
	cachedProjects     []string
	cachedTags         []string
	autocompleteLoaded bool
)

// ensureProjectsAndTags fetches projects and tags from taskwarrior on first
// call and caches the result. Subsequent calls return cached values even if
// the slices are empty (genuinely-empty is not the same as not-yet-loaded).
func ensureProjectsAndTags(projects, tags []string) ([]string, []string) {
	if autocompleteLoaded {
		if len(projects) > 0 {
			return projects, tags
		}
		return cachedProjects, cachedTags
	}
	var err error
	cachedProjects, err = flicktask.GetProjects()
	if err != nil {
		log.Printf("failed to get projects: %v", err)
	}
	cachedTags, err = flicktask.GetTags()
	if err != nil {
		log.Printf("failed to get tags: %v", err)
	}
	autocompleteLoaded = true
	return cachedProjects, cachedTags
}

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
	projects, tags = ensureProjectsAndTags(projects, tags)
	m.updateMatchesWithInput(m.modifyInput.Value(), projects, tags)
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
	projects, tags = ensureProjectsAndTags(projects, tags)
	m.updateSearchMatchesWithInput(m.searchInput.Value(), projects, tags)
}

func (m *Model) updateSearchMatchesWithInput(input string, projects, tags []string) {
	m.updateMatchesWithInput(input, projects, tags)
}
