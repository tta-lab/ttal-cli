package tui

import (
	"testing"
)

func TestSearchAutocompleteFiltersBySearchStr(t *testing.T) {
	// Pre-populate the package-level cache so ensureProjectsAndTags skips the
	// taskwarrior exec call (not available in CI).
	cachedProjects = []string{"ttal", "ttal.cli", "projectX", "other"}
	cachedTags = []string{"bug", "feature"}
	autocompleteLoaded = true
	t.Cleanup(func() {
		cachedProjects = nil
		cachedTags = nil
		autocompleteLoaded = false
	})

	m := Model{
		state:       stateSearch,
		searchStr:   "ttal",
		modifyIndex: 0,
		projects:    []string{"ttal", "ttal.cli", "projectX", "other"},
		tags:        []string{"bug", "feature"},
		modifyInput: "",
	}

	m.updateSearchMatches(m.projects, m.tags)

	t.Logf("searchStr: %q, matches: %v", m.searchStr, m.modifyMatches)

	if len(m.modifyMatches) == 0 {
		t.Fatalf("expected matches but got none")
	}

	hasTtal := false
	hasProjectX := false
	hasOther := false
	for _, match := range m.modifyMatches {
		if match.Type == matchTypeProject && match.Value == "ttal" {
			hasTtal = true
		}
		if match.Type == matchTypeProject && match.Value == "ttal.cli" {
			hasTtal = true
		}
		if match.Type == matchTypeProject && match.Value == "projectX" {
			hasProjectX = true
		}
		if match.Type == matchTypeProject && match.Value == "other" {
			hasOther = true
		}
	}

	if !hasTtal {
		t.Error("expected 'ttal' or 'ttal.cli' in matches")
	}
	if hasProjectX || hasOther {
		t.Error("did NOT expect 'projectX' or 'other' in matches (doesn't contain 'ttal')")
	}
}
