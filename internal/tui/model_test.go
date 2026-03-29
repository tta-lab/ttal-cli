package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyFilterPendingExcludesActiveTasks(t *testing.T) {
	m := Model{
		filter: filterPending,
		tasks: []Task{
			{UUID: "aa000001"},
			{UUID: "bb000002", Start: "20260101T100000Z"},
			{UUID: "cc000003"},
			// scheduled in the past but not active — should remain in pending
			{UUID: "dd000004", Scheduled: "20200101T000000Z"},
		},
		height: 20,
	}
	m.applyFilter()

	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 tasks (active excluded, today-scheduled included), got %d", len(m.filtered))
	}
	for _, task := range m.filtered {
		if task.Start != "" {
			t.Errorf("active task (Start=%q) should be excluded from pending filter", task.Start)
		}
	}
}

func TestApplyFilterPendingAllActive(t *testing.T) {
	m := Model{
		filter: filterPending,
		tasks: []Task{
			{UUID: "aa000001", Start: "20260101T100000Z"},
			{UUID: "bb000002", Start: "20260101T110000Z"},
		},
		height: 20,
	}
	m.applyFilter()

	if len(m.filtered) != 0 {
		t.Fatalf("expected 0 tasks (all active), got %d", len(m.filtered))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0 on empty list, got %d", m.cursor)
	}
}

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

func TestApplyFilter_PassesThroughTasksWithParentID(t *testing.T) {
	m := NewModel()
	m.filter = filterPending
	m.tasks = []Task{
		{UUID: "aaaa-root", Description: "root task", Status: "pending"},
		{UUID: "bbbb-child", Description: "child task", Status: "pending", ParentID: "aaaa-root"},
		{UUID: "cccc-root2", Description: "another root", Status: "pending"},
	}
	m.applyFilter()

	// Root-only filtering is done server-side (parent_id: in loadTasks).
	// applyFilter only handles client-side active/today/completed splitting.
	// This test verifies applyFilter doesn't crash or mishandle tasks with ParentID set.
	assert.Equal(t, 3, len(m.filtered)) // all show when loaded (server handles filtering)
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
