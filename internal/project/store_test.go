package project

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	aliasFbAp = "fb.ap"
	aliasFbTk = "fb.tk"
	nameFbAp  = "Attachment Processor"
	nameFbTk  = "Toolkit"
	pathFbAp  = "/path/fb/ap"
	pathFbTk  = "/path/fb/tk"

	testK8sApp       = "my-api"
	testK8sNamespace = "apps-dev"
	testK8sAppOther  = "my-app"
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

const subPathTOML = `
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

func newSubPathStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.toml")
	if err := os.WriteFile(path, []byte(subPathTOML), 0o644); err != nil {
		t.Fatalf("writing test TOML: %v", err)
	}
	return NewStore(path)
}

func TestStoreSubPathGet(t *testing.T) {
	s := newSubPathStore(t)

	p, err := s.Get(aliasFbAp)
	if err != nil {
		t.Fatalf("Get(%s) error: %v", aliasFbAp, err)
	}
	if p == nil {
		t.Fatalf("Get(%s) returned nil", aliasFbAp)
	}
	if p.Name != nameFbAp {
		t.Errorf("Name = %q, want %q", p.Name, nameFbAp)
	}
	if p.Path != pathFbAp {
		t.Errorf("Path = %q, want %q", p.Path, pathFbAp)
	}
	if p.Alias != aliasFbAp {
		t.Errorf("Alias = %q, want %q", p.Alias, aliasFbAp)
	}

	p, err = s.Get(aliasFbTk)
	if err != nil {
		t.Fatalf("Get(%s) error: %v", aliasFbTk, err)
	}
	if p == nil {
		t.Fatalf("Get(%s) returned nil", aliasFbTk)
	}
	if p.Name != nameFbTk {
		t.Errorf("Name = %q, want %q", p.Name, nameFbTk)
	}
	if p.Path != pathFbTk {
		t.Errorf("Path = %q, want %q", p.Path, pathFbTk)
	}
	if p.Alias != aliasFbTk {
		t.Errorf("Alias = %q, want %q", p.Alias, aliasFbTk)
	}
}

func TestStoreSubPathList(t *testing.T) {
	s := newSubPathStore(t)

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
	if projects[0].Alias != aliasFbAp {
		t.Errorf("projects[0].Alias = %q, want %q", projects[0].Alias, aliasFbAp)
	}
	if projects[1].Alias != aliasFbTk {
		t.Errorf("projects[1].Alias = %q, want %q", projects[1].Alias, aliasFbTk)
	}
	if projects[2].Alias != "ttal" {
		t.Errorf("projects[2].Alias = %q, want %q", projects[2].Alias, "ttal")
	}
}

func TestStoreSubPathRoundTrip(t *testing.T) {
	s := newTestStore(t)

	// Add a dot-notation alias via the store API
	if err := s.Add(aliasFbAp, nameFbAp, pathFbAp); err != nil {
		t.Fatalf("Add(%s) error: %v", aliasFbAp, err)
	}

	// Reload from disk and verify the alias survives the round-trip
	p, err := s.Get(aliasFbAp)
	if err != nil {
		t.Fatalf("Get(%s) after reload error: %v", aliasFbAp, err)
	}
	if p == nil {
		t.Fatalf("Get(%s) returned nil after reload", aliasFbAp)
	}
	if p.Name != nameFbAp {
		t.Errorf("Name = %q, want %q", p.Name, nameFbAp)
	}
	if p.Path != pathFbAp {
		t.Errorf("Path = %q, want %q", p.Path, pathFbAp)
	}
	if p.Alias != aliasFbAp {
		t.Errorf("Alias = %q, want %q", p.Alias, aliasFbAp)
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
	p, err := s.Get(aliasFbAp) // Get only checks active
	if err != nil {
		t.Fatalf("Get(%s) error: %v", aliasFbAp, err)
	}
	if p != nil {
		t.Errorf("archived %s should not appear in active Get()", aliasFbAp)
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
	if archived[0].Alias != aliasFbAp {
		t.Errorf("archived[0].Alias = %q, want %q", archived[0].Alias, aliasFbAp)
	}
	if archived[0].Name != nameFbAp {
		t.Errorf("archived[0].Name = %q, want %q", archived[0].Name, nameFbAp)
	}
	if archived[1].Alias != aliasFbTk {
		t.Errorf("archived[1].Alias = %q, want %q", archived[1].Alias, aliasFbTk)
	}

	// Unarchive should work
	if err := s.Unarchive(aliasFbAp); err != nil {
		t.Fatalf("Unarchive(%s) error: %v", aliasFbAp, err)
	}
	p, err = s.Get(aliasFbAp)
	if err != nil {
		t.Fatalf("Get(%s) after unarchive error: %v", aliasFbAp, err)
	}
	if p == nil {
		t.Fatalf("%s should be active after unarchive", aliasFbAp)
	}
}

func TestStoreGitHubTokenEnvRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")
	s := NewStore(path)

	if err := s.Add("guion", "Guion", "/path/guion"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := s.Modify("guion", map[string]string{"github_token_env": testTokenEnvVar}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	// Reload via fresh store — catches serialization bugs that in-memory tests miss
	s2 := NewStore(path)
	p, err := s2.Get("guion")
	if err != nil {
		t.Fatalf("Get() after reload error: %v", err)
	}
	if p == nil {
		t.Fatal("Get() returned nil after reload")
	}
	if p.GitHubTokenEnv != testTokenEnvVar {
		t.Errorf("GitHubTokenEnv = %q, want %q", p.GitHubTokenEnv, testTokenEnvVar)
	}
}

func TestStoreGitHubTokenEnvPreservation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")
	s := NewStore(path)

	// Add project A with github_token_env
	if err := s.Add("guion", "Guion", "/path/guion"); err != nil {
		t.Fatalf("Add(guion) error: %v", err)
	}
	if err := s.Modify("guion", map[string]string{"github_token_env": testTokenEnvVar}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	// Adding project B triggers save()
	if err := s.Add("other", "Other", "/path/other"); err != nil {
		t.Fatalf("Add(other) error: %v", err)
	}

	// Reload via fresh store — guion's github_token_env must still be set
	s2 := NewStore(path)
	p, err := s2.Get("guion")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if p == nil {
		t.Fatal("Get() returned nil")
	}
	if p.GitHubTokenEnv != testTokenEnvVar {
		t.Errorf("GitHubTokenEnv = %q after second project added, want %q", p.GitHubTokenEnv, testTokenEnvVar)
	}
}

func TestStoreFlattenDoesNotTreatGitHubTokenEnvAsSubProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")

	tomlContent := "[guion]\nname = \"Guion\"\npath = \"/path/guion\"\ngithub_token_env = \"" + testTokenEnvVar + "\"\n"
	if err := os.WriteFile(path, []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("writing test TOML: %v", err)
	}

	s := NewStore(path)
	projects, err := s.List(false)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	// Should be exactly 1 project — github_token_env must not be treated as sub-project
	if len(projects) != 1 {
		aliases := make([]string, len(projects))
		for i, p := range projects {
			aliases[i] = p.Alias
		}
		t.Fatalf("List() returned %d projects, want 1: %v", len(projects), aliases)
	}
	if projects[0].GitHubTokenEnv != testTokenEnvVar {
		t.Errorf("GitHubTokenEnv = %q, want %q", projects[0].GitHubTokenEnv, testTokenEnvVar)
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

func TestStoreModifyK8sFields(t *testing.T) {
	s := newTestStore(t)
	mustAdd(t, s, "proj", "Project", "/path")

	if err := s.Modify("proj", map[string]string{"k8s_app": testK8sApp, "k8s_namespace": testK8sNamespace}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	p, _ := s.Get("proj")
	if p.K8sApp != testK8sApp {
		t.Errorf("K8sApp = %q, want %q", p.K8sApp, testK8sApp)
	}
	if p.K8sNamespace != testK8sNamespace {
		t.Errorf("K8sNamespace = %q, want %q", p.K8sNamespace, testK8sNamespace)
	}
}

func TestStoreK8sFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")
	s := NewStore(path)

	if err := s.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := s.Modify("proj", map[string]string{"k8s_app": testK8sApp, "k8s_namespace": testK8sNamespace}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	// Reload via fresh store and verify fields survive
	s2 := NewStore(path)
	p, err := s2.Get("proj")
	if err != nil {
		t.Fatalf("Get() after reload error: %v", err)
	}
	if p == nil {
		t.Fatal("Get() returned nil after reload")
	}
	if p.K8sApp != testK8sApp {
		t.Errorf("K8sApp = %q, want %q", p.K8sApp, testK8sApp)
	}
	if p.K8sNamespace != testK8sNamespace {
		t.Errorf("K8sNamespace = %q, want %q", p.K8sNamespace, testK8sNamespace)
	}
}

func TestStoreModifyPreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")
	s := NewStore(path)

	if err := s.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	// Set github_token_env
	if err := s.Modify("proj", map[string]string{"github_token_env": "MY_TOKEN"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	// Modify k8s fields — github_token_env must survive
	if err := s.Modify("proj", map[string]string{
		"k8s_app":       testK8sAppOther,
		"k8s_namespace": testK8sNamespace,
	}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	s2 := NewStore(path)
	p, _ := s2.Get("proj")
	if p.GitHubTokenEnv != "MY_TOKEN" {
		t.Errorf("GitHubTokenEnv = %q, want %q", p.GitHubTokenEnv, "MY_TOKEN")
	}
	if p.K8sApp != testK8sAppOther {
		t.Errorf("K8sApp = %q, want %q", p.K8sApp, testK8sAppOther)
	}
	if p.K8sNamespace != testK8sNamespace {
		t.Errorf("K8sNamespace = %q, want %q", p.K8sNamespace, testK8sNamespace)
	}

	// Modify github_token_env — k8s fields must survive
	if err := s.Modify("proj", map[string]string{"github_token_env": "OTHER_TOKEN"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	s3 := NewStore(path)
	p, _ = s3.Get("proj")
	if p.GitHubTokenEnv != "OTHER_TOKEN" {
		t.Errorf("GitHubTokenEnv = %q, want %q", p.GitHubTokenEnv, "OTHER_TOKEN")
	}
	if p.K8sApp != testK8sAppOther {
		t.Errorf("K8sApp = %q, want %q", p.K8sApp, testK8sAppOther)
	}
	if p.K8sNamespace != testK8sNamespace {
		t.Errorf("K8sNamespace = %q, want %q", p.K8sNamespace, testK8sNamespace)
	}
}

func TestStoreFlattenDoesNotTreatK8sFieldsAsSubProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.toml")

	tomlContent := `[proj]
name = "Project"
path = "/path/proj"
k8s_app = "` + testK8sAppOther + `"
k8s_namespace = "` + testK8sNamespace + `"
`
	if err := os.WriteFile(path, []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("writing test TOML: %v", err)
	}

	s := NewStore(path)
	projects, err := s.List(false)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	// Should be exactly 1 project — k8s_app and k8s_namespace must not be treated as sub-projects
	if len(projects) != 1 {
		aliases := make([]string, len(projects))
		for i, p := range projects {
			aliases[i] = p.Alias
		}
		t.Fatalf("List() returned %d projects, want 1: %v", len(projects), aliases)
	}
	if projects[0].K8sApp != "my-app" {
		t.Errorf("K8sApp = %q, want %q", projects[0].K8sApp, "my-app")
	}
	if projects[0].K8sNamespace != "apps-dev" {
		t.Errorf("K8sNamespace = %q, want %q", projects[0].K8sNamespace, "apps-dev")
	}
}
