package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const twExportArg = "export"

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

func TestApplyFilterActive_IncludesStartedSubtasks(t *testing.T) {
	// repro: f7e395e6 has parent_id + Start set but was filtered out of active list
	// because buildLoadTasksArgs added parent_id: restriction for filterActive.
	// The active view is flat (showActive=true) — subtasks with Start set should appear.
	m := NewModel()
	m.filter = filterActive
	m.tasks = []Task{
		// applyFilter requires Start != "" for active view
		{UUID: "root-1", Description: "root task", Status: "pending", Start: "20260409T120000"},
		{UUID: "child-1", Description: "started subtask", Status: "pending", ParentID: "root-1", Start: "20260409T120000"},
		{UUID: "root-2", Description: "another root", Status: "pending", Start: "20260409T120000"},
	}
	m.applyFilter()

	// All three tasks should appear — active filter is flat and includes subtasks
	assert.Equal(t, 3, len(m.filtered))

	// Negative case: tasks without Start are excluded from active filter
	m2 := NewModel()
	m2.filter = filterActive
	m2.tasks = []Task{
		{UUID: "not-started", Description: "pending but not started", Status: "pending", Start: ""},
		{UUID: "started-root", Description: "started root", Status: "pending", Start: "20260409T120000"},
	}
	m2.applyFilter()
	assert.Equal(t, 1, len(m2.filtered))
	assert.Equal(t, "started-root", m2.filtered[0].UUID)
}

func TestBuildLoadTasksArgs_PendingAndToday(t *testing.T) {
	for _, filter := range []filterMode{filterPending, filterToday} {
		args := buildLoadTasksArgs(filter, "")
		if len(args) < 2 {
			t.Fatalf("filter %v: expected at least [status:pending, ...], got %v", filter, args)
		}
		if args[0] != "status:pending" {
			t.Errorf("filter %v: expected status:pending first, got %v", filter, args[0])
		}
		if args[len(args)-1] != twExportArg {
			t.Errorf("filter %v: expected export last, got %v", filter, args[len(args)-1])
		}
	}
}

func TestBuildLoadTasksArgs_Active(t *testing.T) {
	// filterActive: status:pending, no parent_id restriction (flat view includes subtasks)
	argsActive := buildLoadTasksArgs(filterActive, "")
	if argsActive[0] != "status:pending" {
		t.Errorf("filterActive: expected status:pending first, got %v", argsActive[0])
	}
	if argsActive[len(argsActive)-1] != twExportArg {
		t.Errorf("filterActive: expected export last, got %v", argsActive[len(argsActive)-1])
	}
	// filterActive must not include parent_id: — that would exclude subtasks like f7e395e6
	for _, arg := range argsActive {
		if arg == "parent_id:" {
			t.Errorf("filterActive: should not include parent_id:, got %v", argsActive)
		}
	}

	// filterActive with search: search terms pass through correctly
	argsActiveWithSearch := buildLoadTasksArgs(filterActive, "project:ttal")
	if argsActiveWithSearch[len(argsActiveWithSearch)-2] != "project:ttal" {
		t.Errorf("expected search arg before export, got %v", argsActiveWithSearch)
	}
}

func TestBuildLoadTasksArgs_Completed(t *testing.T) {
	// filterCompleted: root tasks only (tree view), includes parent_id: when IsFork
	argsCompleted := buildLoadTasksArgs(filterCompleted, "")
	if argsCompleted[0] != "status:completed" {
		t.Errorf("filterCompleted: expected status:completed first, got %v", argsCompleted[0])
	}
	if argsCompleted[len(argsCompleted)-1] != twExportArg {
		t.Errorf("filterCompleted: expected export last, got %v", argsCompleted[len(argsCompleted)-1])
	}
	// filterCompleted includes parent_id: when IsFork (hides completed subtasks from view)
	if taskwarrior.IsFork() {
		hasParentID := false
		for _, arg := range argsCompleted {
			if arg == "parent_id:" {
				hasParentID = true
				break
			}
		}
		if !hasParentID {
			t.Errorf("filterCompleted: expected parent_id: when IsFork, got %v", argsCompleted)
		}
	}
}

func TestBuildLoadTasksArgs_SearchPassthrough(t *testing.T) {
	argsWithSearch := buildLoadTasksArgs(filterPending, "project:ttal")
	if argsWithSearch[len(argsWithSearch)-2] != "project:ttal" {
		t.Errorf("expected search arg before export, got %v", argsWithSearch)
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

func TestApplyFilterCompletedSortsByEndDescending(t *testing.T) {
	m := Model{
		filter: filterCompleted,
		tasks: []Task{
			{UUID: "aa000001", Status: "completed", End: "20260101T100000Z"},
			{UUID: "bb000002", Status: "completed", End: "20260315T120000Z"},
			{UUID: "cc000003", Status: "completed", End: "20260210T080000Z"},
		},
		height: 20,
	}
	m.applyFilter()
	assert.Equal(t, 3, len(m.filtered))
	assert.Equal(t, "bb000002", m.filtered[0].UUID, "most recent should be first")
	assert.Equal(t, "cc000003", m.filtered[1].UUID, "second most recent should be second")
	assert.Equal(t, "aa000001", m.filtered[2].UUID, "oldest should be last")
}
