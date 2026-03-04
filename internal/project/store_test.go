package project

import (
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
