package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/project"
)

const (
	testNewName = "New Name"
	testNewPath = "/new/path"
)

func newTestStore(t *testing.T) *project.Store {
	t.Helper()
	return project.NewStore(filepath.Join(t.TempDir(), "projects.toml"))
}

func TestProjectModifyAlias(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("old-alias", "Test Project", ""); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	if err := store.Modify("old-alias", map[string]string{"alias": "new-alias"}); err != nil {
		t.Fatalf("failed to update project alias: %v", err)
	}

	p, err := store.Get("old-alias")
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if p != nil {
		t.Error("old alias should not exist after rename")
	}

	updated, err := store.Get("new-alias")
	if err != nil {
		t.Fatalf("failed to query project by new alias: %v", err)
	}
	if updated == nil {
		t.Fatal("project with new alias not found")
		return
	}
	if updated.Alias != "new-alias" {
		t.Errorf("project alias = %v, want new-alias", updated.Alias)
	}
}

func TestProjectModifyName(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("test-proj", "Old Name", ""); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	if err := store.Modify("test-proj", map[string]string{"name": testNewName}); err != nil {
		t.Fatalf("failed to update project name: %v", err)
	}

	updated, err := store.Get("test-proj")
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if updated == nil {
		t.Fatal("project not found")
		return
	}
	if updated.Name != testNewName {
		t.Errorf("project name = %v, want %v", updated.Name, testNewName)
	}
}

func TestProjectModifyPath(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("test-proj", "Test Project", "/old/path"); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	if err := store.Modify("test-proj", map[string]string{"path": testNewPath}); err != nil {
		t.Fatalf("failed to update project path: %v", err)
	}

	updated, err := store.Get("test-proj")
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if updated == nil {
		t.Fatal("project not found")
		return
	}
	if updated.Path != testNewPath {
		t.Errorf("project path = %v, want %v", updated.Path, testNewPath)
	}
}

func TestProjectModifyMultipleFields(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("test-proj", "Old Name", "/old/path"); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	if err := store.Modify("test-proj", map[string]string{"name": testNewName, "path": testNewPath}); err != nil {
		t.Fatalf("failed to update project: %v", err)
	}

	updated, err := store.Get("test-proj")
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if updated == nil {
		t.Fatal("project not found")
		return
	}
	if updated.Name != testNewName {
		t.Errorf("project name = %v, want New Name", updated.Name)
	}
	if updated.Path != testNewPath {
		t.Errorf("project path = %v, want /new/path", updated.Path)
	}
}

func TestProjectArchive(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("test-proj", "Test Project", ""); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Verify it's active
	p, _ := store.Get("test-proj")
	if p == nil {
		t.Fatal("project should be active after creation")
	}

	// Archive it
	if err := store.Archive("test-proj"); err != nil {
		t.Fatalf("failed to archive project: %v", err)
	}

	// Should no longer appear in active
	p, _ = store.Get("test-proj")
	if p != nil {
		t.Error("project should not be active after archiving")
	}

	// Should appear in archived
	archived, _ := store.List(true)
	if len(archived) != 1 || archived[0].Alias != "test-proj" {
		t.Error("project should appear in archived list")
	}

	// Unarchive it
	if err := store.Unarchive("test-proj"); err != nil {
		t.Fatalf("failed to unarchive project: %v", err)
	}

	// Should be active again
	p, _ = store.Get("test-proj")
	if p == nil {
		t.Error("project should be active after unarchiving")
	}
}

func TestProjectDelete(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("to-delete", "Delete Me", ""); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	if err := store.Delete("to-delete"); err != nil {
		t.Fatalf("failed to delete project: %v", err)
	}

	p, err := store.Get("to-delete")
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if p != nil {
		t.Error("project should not exist after deletion")
	}
}

func TestProjectDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.Delete("nonexistent")
	if err == nil {
		t.Error("deleting nonexistent project should return error")
	}
}

func TestProjectListArchivedOnly(t *testing.T) {
	store := newTestStore(t)

	if err := store.Add("active-proj", "Active", ""); err != nil {
		t.Fatalf("failed to create active project: %v", err)
	}
	if err := store.Add("archived-proj", "Archived", ""); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	if err := store.Archive("archived-proj"); err != nil {
		t.Fatalf("failed to archive project: %v", err)
	}

	archived, err := store.List(true)
	if err != nil {
		t.Fatalf("failed to list archived projects: %v", err)
	}
	if len(archived) != 1 {
		t.Errorf("found %d archived projects, want 1", len(archived))
	}
	if len(archived) > 0 && archived[0].Alias != "archived-proj" {
		t.Errorf("archived project alias = %v, want archived-proj", archived[0].Alias)
	}

	active, err := store.List(false)
	if err != nil {
		t.Fatalf("failed to list active projects: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("found %d active projects, want 1", len(active))
	}
	if len(active) > 0 && active[0].Alias != "active-proj" {
		t.Errorf("active project alias = %v, want active-proj", active[0].Alias)
	}
}

func TestProjectListJSON(t *testing.T) {
	store := newTestStore(t)
	if err := store.Add("proj1", "Project One", "/path/one"); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	if err := store.Add("proj2", "Project Two", "/path/two"); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	projects, err := store.List(false)
	if err != nil {
		t.Fatalf("failed to list projects: %v", err)
	}

	// Reproduce the JSON output logic from the command
	type projectJSON struct {
		Alias string `json:"alias"`
		Name  string `json:"name"`
		Path  string `json:"path"`
	}
	output := make([]projectJSON, 0, len(projects))
	for _, p := range projects {
		output = append(output, projectJSON{Alias: p.Alias, Name: p.Name, Path: p.Path})
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal projects: %v", err)
	}

	// Parse JSON output and verify structure
	var results []map[string]string
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, string(data))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 projects in JSON, got %d", len(results))
	}
	assertJSONProjectFields(t, results)
}

// assertJSONProjectFields checks that each item has alias, name, path keys
// and that proj1 is present with expected values.
func assertJSONProjectFields(t *testing.T, results []map[string]string) {
	t.Helper()
	requiredFields := []string{"alias", "name", "path"}
	found := false
	for _, r := range results {
		for _, field := range requiredFields {
			if _, ok := r[field]; !ok {
				t.Errorf("JSON object missing %q field", field)
			}
		}
		if r["alias"] == "proj1" && r["name"] == "Project One" && r["path"] == "/path/one" {
			found = true
		}
	}
	if !found {
		t.Error("expected project proj1 not found in JSON output")
	}
}
