package tui

import (
	"testing"
)

func TestSearchInputStartsEmpty(t *testing.T) {
	m := NewModel()
	if m.searchInput.Value() != "" {
		t.Errorf("searchInput should start empty, got %q", m.searchInput.Value())
	}
}

func TestAnnotateInputAcceptsText(t *testing.T) {
	m := NewModel()
	m.annotateInput.SetValue("test annotation")
	if m.annotateInput.Value() != "test annotation" {
		t.Errorf("expected %q, got %q", "test annotation", m.annotateInput.Value())
	}
}

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

	searchInput := newTextInput("placeholder")
	searchInput.SetValue("ttal")
	modifyInput := newTextInput("placeholder")

	m := Model{
		state:       stateSearch,
		searchInput: searchInput,
		modifyIndex: 0,
		projects:    []string{"ttal", "ttal.cli", "projectX", "other"},
		tags:        []string{"bug", "feature"},
		modifyInput: modifyInput,
	}

	m.updateSearchMatches(m.projects, m.tags)

	t.Logf("searchStr: %q, matches: %v", m.searchInput.Value(), m.modifyMatches)

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
