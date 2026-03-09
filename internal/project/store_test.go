package project

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "projects.toml"))
}

func mustAdd(t *testing.T, s *Store, alias, name, path string) {
	t.Helper()
	if err := s.Add(alias, name, path); err != nil {
		t.Fatalf("Add(%q) error: %v", alias, err)
	}
}

func TestStoreAddAndGet(t *testing.T) {
	s := newTestStore(t)

	if err := s.Add("ttal", "TTAL Core", "/path/ttal"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	p, err := s.Get("ttal")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if p == nil {
		t.Fatal("Get() returned nil")
	}
	if p.Name != "TTAL Core" {
		t.Errorf("Name = %q, want %q", p.Name, "TTAL Core")
	}
	if p.Path != "/path/ttal" {
		t.Errorf("Path = %q, want %q", p.Path, "/path/ttal")
	}
	if p.Alias != "ttal" {
		t.Errorf("Alias = %q, want %q", p.Alias, "ttal")
	}
}

func TestStoreAddDuplicate(t *testing.T) {
	s := newTestStore(t)

	if err := s.Add("ttal", "TTAL", ""); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	if err := s.Add("ttal", "TTAL Again", ""); err == nil {
		t.Fatal("second Add() should return error for duplicate alias")
	}
}

func TestStoreGetNotFound(t *testing.T) {
	s := newTestStore(t)

	p, err := s.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if p != nil {
		t.Error("Get() should return nil for nonexistent alias")
	}
}

func TestStoreList(t *testing.T) {
	s := newTestStore(t)

	mustAdd(t, s, "aaa", "AAA", "/a")
	mustAdd(t, s, "bbb", "BBB", "/b")

	projects, err := s.List(false)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("List() returned %d projects, want 2", len(projects))
	}
	// Should be sorted by alias
	if projects[0].Alias != "aaa" || projects[1].Alias != "bbb" {
		t.Errorf("List() not sorted: got %q, %q", projects[0].Alias, projects[1].Alias)
	}
}

func TestStoreArchiveUnarchive(t *testing.T) {
	s := newTestStore(t)
	mustAdd(t, s, "proj", "Project", "/path")

	if err := s.Archive("proj"); err != nil {
		t.Fatalf("Archive() error: %v", err)
	}

	// Should not be in active list
	active, _ := s.List(false)
	if len(active) != 0 {
		t.Error("archived project should not appear in active list")
	}

	// Should be in archived list
	archived, _ := s.List(true)
	if len(archived) != 1 {
		t.Fatalf("List(archived) returned %d, want 1", len(archived))
	}
	if archived[0].Alias != "proj" {
		t.Errorf("archived alias = %q, want %q", archived[0].Alias, "proj")
	}

	// Unarchive
	if err := s.Unarchive("proj"); err != nil {
		t.Fatalf("Unarchive() error: %v", err)
	}

	active, _ = s.List(false)
	if len(active) != 1 {
		t.Error("unarchived project should appear in active list")
	}
}

func TestStoreDelete(t *testing.T) {
	s := newTestStore(t)
	mustAdd(t, s, "proj", "Project", "/path")

	if err := s.Delete("proj"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	p, _ := s.Get("proj")
	if p != nil {
		t.Error("deleted project should not be found")
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.Delete("nonexistent"); err == nil {
		t.Error("Delete() should return error for nonexistent alias")
	}
}

func TestStoreModify(t *testing.T) {
	s := newTestStore(t)
	mustAdd(t, s, "proj", "Old Name", "/old/path")

	if err := s.Modify("proj", map[string]string{"name": "New Name", "path": "/new/path"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	p, _ := s.Get("proj")
	if p.Name != "New Name" {
		t.Errorf("Name = %q, want %q", p.Name, "New Name")
	}
	if p.Path != "/new/path" {
		t.Errorf("Path = %q, want %q", p.Path, "/new/path")
	}
}

func TestStoreModifyAlias(t *testing.T) {
	s := newTestStore(t)
	mustAdd(t, s, "old", "Project", "/path")

	if err := s.Modify("old", map[string]string{"alias": "new"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	p, _ := s.Get("old")
	if p != nil {
		t.Error("old alias should not exist")
	}

	p, _ = s.Get("new")
	if p == nil {
		t.Fatal("new alias should exist")
	}
}

func TestStoreFileCreatedOnFirstWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "projects.toml")
	s := NewStore(path)

	// Get on nonexistent file should return nil, no error
	p, err := s.Get("anything")
	if err != nil {
		t.Fatalf("Get() on missing file: %v", err)
	}
	if p != nil {
		t.Error("Get() should return nil on missing file")
	}

	// Add should create the file
	if err := s.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	p, _ = s.Get("proj")
	if p == nil {
		t.Fatal("project should exist after Add()")
	}
}

func TestStoreSubPathProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")

	// Write TOML with dot-notation nested tables like [fb.ap] and [fb.tk]
	tomlContent := `
[fb.ap]
name = "Attachment Processor"
path = "/path/fb/ap"

[fb.tk]
name = "Toolkit"
path = "/path/fb/tk"

[ttal]
name = "TTAL Core"
path = "/path/ttal"
`
	if err := os.WriteFile(path, []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("writing test TOML: %v", err)
	}

	s := NewStore(path)

	// Get sub-path projects
	p, err := s.Get("fb.ap")
	if err != nil {
		t.Fatalf("Get(fb.ap) error: %v", err)
	}
	if p == nil {
		t.Fatal("Get(fb.ap) returned nil")
	}
	if p.Name != "Attachment Processor" {
		t.Errorf("Name = %q, want %q", p.Name, "Attachment Processor")
	}
	if p.Path != "/path/fb/ap" {
		t.Errorf("Path = %q, want %q", p.Path, "/path/fb/ap")
	}
	if p.Alias != "fb.ap" {
		t.Errorf("Alias = %q, want %q", p.Alias, "fb.ap")
	}

	p, err = s.Get("fb.tk")
	if err != nil {
		t.Fatalf("Get(fb.tk) error: %v", err)
	}
	if p == nil {
		t.Fatal("Get(fb.tk) returned nil")
	}
	if p.Name != "Toolkit" {
		t.Errorf("fb.tk Name = %q, want %q", p.Name, "Toolkit")
	}
	if p.Path != "/path/fb/tk" {
		t.Errorf("fb.tk Path = %q, want %q", p.Path, "/path/fb/tk")
	}
	if p.Alias != "fb.tk" {
		t.Errorf("fb.tk Alias = %q, want %q", p.Alias, "fb.tk")
	}

	// List should include all three projects
	projects, err := s.List(false)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(projects) != 3 {
		aliases := make([]string, len(projects))
		for i, p := range projects {
			aliases[i] = p.Alias
		}
		t.Fatalf("List() returned %d projects, want 3: %v", len(projects), aliases)
	}

	// Verify sorted order: fb.ap, fb.tk, ttal
	if projects[0].Alias != "fb.ap" {
		t.Errorf("projects[0].Alias = %q, want %q", projects[0].Alias, "fb.ap")
	}
	if projects[1].Alias != "fb.tk" {
		t.Errorf("projects[1].Alias = %q, want %q", projects[1].Alias, "fb.tk")
	}
	if projects[2].Alias != "ttal" {
		t.Errorf("projects[2].Alias = %q, want %q", projects[2].Alias, "ttal")
	}
}

func TestStoreSubPathRoundTrip(t *testing.T) {
	s := newTestStore(t)

	// Add a dot-notation alias via the store API
	if err := s.Add("fb.ap", "Attachment Processor", "/path/fb/ap"); err != nil {
		t.Fatalf("Add(fb.ap) error: %v", err)
	}

	// Reload from disk and verify the alias survives the round-trip
	p, err := s.Get("fb.ap")
	if err != nil {
		t.Fatalf("Get(fb.ap) after reload error: %v", err)
	}
	if p == nil {
		t.Fatal("Get(fb.ap) returned nil after reload")
	}
	if p.Name != "Attachment Processor" {
		t.Errorf("Name = %q, want %q", p.Name, "Attachment Processor")
	}
	if p.Path != "/path/fb/ap" {
		t.Errorf("Path = %q, want %q", p.Path, "/path/fb/ap")
	}
	if p.Alias != "fb.ap" {
		t.Errorf("Alias = %q, want %q", p.Alias, "fb.ap")
	}
}

func TestStoreArchivedSubPathProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")

	// Write TOML with archived dot-notation entries
	tomlContent := `
[ttal]
name = "TTAL Core"
path = "/path/ttal"

[archived.fb.ap]
name = "Attachment Processor"
path = "/path/fb/ap"

[archived.fb.tk]
name = "Toolkit"
path = "/path/fb/tk"
`
	if err := os.WriteFile(path, []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("writing test TOML: %v", err)
	}

	s := NewStore(path)

	// Archived dot-notation projects should be present
	p, err := s.Get("fb.ap") // Get only checks active
	if err != nil {
		t.Fatalf("Get(fb.ap) error: %v", err)
	}
	if p != nil {
		t.Error("archived fb.ap should not appear in active Get()")
	}

	archived, err := s.List(true)
	if err != nil {
		t.Fatalf("List(archived) error: %v", err)
	}
	if len(archived) != 2 {
		aliases := make([]string, len(archived))
		for i, p := range archived {
			aliases[i] = p.Alias
		}
		t.Fatalf("List(archived) returned %d projects, want 2: %v", len(archived), aliases)
	}

	// Verify aliases
	if archived[0].Alias != "fb.ap" {
		t.Errorf("archived[0].Alias = %q, want %q", archived[0].Alias, "fb.ap")
	}
	if archived[0].Name != "Attachment Processor" {
		t.Errorf("archived[0].Name = %q, want %q", archived[0].Name, "Attachment Processor")
	}
	if archived[1].Alias != "fb.tk" {
		t.Errorf("archived[1].Alias = %q, want %q", archived[1].Alias, "fb.tk")
	}

	// Unarchive should work
	if err := s.Unarchive("fb.ap"); err != nil {
		t.Fatalf("Unarchive(fb.ap) error: %v", err)
	}
	p, err = s.Get("fb.ap")
	if err != nil {
		t.Fatalf("Get(fb.ap) after unarchive error: %v", err)
	}
	if p == nil {
		t.Fatal("fb.ap should be active after unarchive")
	}
}

func TestStoreExists(t *testing.T) {
	s := newTestStore(t)
	mustAdd(t, s, "active", "Active", "")
	mustAdd(t, s, "will-archive", "Will Archive", "")
	if err := s.Archive("will-archive"); err != nil {
		t.Fatalf("Archive() error: %v", err)
	}

	exists, _ := s.Exists("active")
	if !exists {
		t.Error("active project should exist")
	}

	exists, _ = s.Exists("will-archive")
	if !exists {
		t.Error("archived project should exist")
	}

	exists, _ = s.Exists("nonexistent")
	if exists {
		t.Error("nonexistent project should not exist")
	}
}
