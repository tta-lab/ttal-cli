package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Project represents a project entry.
type Project struct {
	Name     string `toml:"name"`
	Path     string `toml:"path"`
	Alias    string `toml:"-"` // derived from TOML key
	Archived bool   `toml:"-"` // derived from section
}

// projectEntry is the on-disk TOML structure for a single project.
type projectEntry struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// projectsFile is the on-disk TOML structure.
type projectsFile struct {
	Active   map[string]projectEntry `toml:"-"`
	Archived map[string]projectEntry `toml:"archived"`
}

// Store manages project TOML files.
type Store struct {
	path string
}

// NewStore creates a store for the given TOML file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the store file path.
func (s *Store) Path() string {
	return s.path
}

// load reads the projects file, returning empty maps if it doesn't exist.
func (s *Store) load() (*projectsFile, error) {
	pf := &projectsFile{
		Active:   make(map[string]projectEntry),
		Archived: make(map[string]projectEntry),
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return pf, nil
		}
		return nil, fmt.Errorf("reading projects file: %w", err)
	}

	// Decode into a raw map first to separate active keys from [archived].
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing projects file: %w", err)
	}

	for key, val := range raw {
		if key == "archived" {
			archivedMap, ok := val.(map[string]any)
			if !ok {
				continue
			}
			for alias, v := range archivedMap {
				pf.Archived[alias] = parseEntry(v)
			}
			continue
		}
		flattenProjects(pf.Active, key, val)
	}

	return pf, nil
}

// flattenProjects recursively extracts project entries from nested TOML tables.
// [fb.ap] in TOML becomes key "ap" inside fb's map — this flattens it to "fb.ap".
func flattenProjects(out map[string]projectEntry, prefix string, val any) {
	m, ok := val.(map[string]any)
	if !ok {
		return
	}
	if _, hasName := m["name"]; hasName {
		out[prefix] = parseEntry(val)
	}
	for k, v := range m {
		if k == "name" || k == "path" {
			continue
		}
		if _, ok := v.(map[string]any); ok {
			flattenProjects(out, prefix+"."+k, v)
		}
	}
}

func parseEntry(val any) projectEntry {
	m, ok := val.(map[string]any)
	if !ok {
		return projectEntry{}
	}
	e := projectEntry{}
	if name, ok := m["name"].(string); ok {
		e.Name = name
	}
	if path, ok := m["path"].(string); ok {
		e.Path = path
	}
	return e
}

// save writes the projects file atomically using temp-file + rename.
func (s *Store) save(pf *projectsFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp) // clean up on any failure; no-op after successful rename

	enc := toml.NewEncoder(f)

	// Write active projects sorted by alias.
	aliases := sortedKeys(pf.Active)
	for _, alias := range aliases {
		entry := pf.Active[alias]
		m := map[string]map[string]string{
			alias: entryToMap(entry),
		}
		if err := enc.Encode(m); err != nil {
			f.Close()
			return fmt.Errorf("encoding active project %s: %w", alias, err)
		}
	}

	// Write archived section if non-empty.
	if len(pf.Archived) > 0 {
		archived := map[string]map[string]map[string]string{
			"archived": {},
		}
		for alias, entry := range pf.Archived {
			archived["archived"][alias] = entryToMap(entry)
		}
		if err := enc.Encode(archived); err != nil {
			f.Close()
			return fmt.Errorf("encoding archived projects: %w", err)
		}
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	return os.Rename(tmp, s.path)
}

func entryToMap(e projectEntry) map[string]string {
	m := map[string]string{"name": e.Name}
	if e.Path != "" {
		m["path"] = e.Path
	}
	return m
}

func sortedKeys(m map[string]projectEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// List returns all projects. If archived is true, returns only archived; otherwise only active.
func (s *Store) List(archived bool) ([]Project, error) {
	pf, err := s.load()
	if err != nil {
		return nil, err
	}

	var source map[string]projectEntry
	if archived {
		source = pf.Archived
	} else {
		source = pf.Active
	}

	projects := make([]Project, 0, len(source))
	for alias, entry := range source {
		projects = append(projects, Project{
			Name:     entry.Name,
			Path:     entry.Path,
			Alias:    alias,
			Archived: archived,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Alias < projects[j].Alias
	})

	return projects, nil
}

// Get returns a project by alias (active only). Returns nil if not found.
func (s *Store) Get(alias string) (*Project, error) {
	pf, err := s.load()
	if err != nil {
		return nil, err
	}

	entry, ok := pf.Active[alias]
	if !ok {
		return nil, nil
	}

	return &Project{
		Name:  entry.Name,
		Path:  entry.Path,
		Alias: alias,
	}, nil
}

// Add creates a new project. Returns error if alias already exists.
func (s *Store) Add(alias, name, path string) error {
	pf, err := s.load()
	if err != nil {
		return err
	}

	if _, ok := pf.Active[alias]; ok {
		return fmt.Errorf("project %q already exists", alias)
	}
	if _, ok := pf.Archived[alias]; ok {
		return fmt.Errorf("project %q already exists (archived)", alias)
	}

	pf.Active[alias] = projectEntry{Name: name, Path: path}
	return s.save(pf)
}

// Modify updates fields on an existing active project.
func (s *Store) Modify(alias string, updates map[string]string) error {
	pf, err := s.load()
	if err != nil {
		return err
	}

	entry, ok := pf.Active[alias]
	if !ok {
		return fmt.Errorf("project %q not found", alias)
	}

	newAlias := alias
	for field, value := range updates {
		switch field {
		case "alias":
			newAlias = value
		case "name":
			entry.Name = value
		case "path":
			entry.Path = value
		default:
			return fmt.Errorf("unknown field %q (available: alias, name, path)", field)
		}
	}

	if newAlias != alias {
		if _, exists := pf.Active[newAlias]; exists {
			return fmt.Errorf("project %q already exists", newAlias)
		}
		delete(pf.Active, alias)
	}
	pf.Active[newAlias] = entry

	return s.save(pf)
}

// Archive moves a project from active to archived.
func (s *Store) Archive(alias string) error {
	pf, err := s.load()
	if err != nil {
		return err
	}

	entry, ok := pf.Active[alias]
	if !ok {
		return fmt.Errorf("project %q not found", alias)
	}

	delete(pf.Active, alias)
	pf.Archived[alias] = entry
	return s.save(pf)
}

// Unarchive moves a project from archived back to active.
func (s *Store) Unarchive(alias string) error {
	pf, err := s.load()
	if err != nil {
		return err
	}

	entry, ok := pf.Archived[alias]
	if !ok {
		return fmt.Errorf("archived project %q not found", alias)
	}

	delete(pf.Archived, alias)
	pf.Active[alias] = entry
	return s.save(pf)
}

// Delete permanently removes a project from either section.
func (s *Store) Delete(alias string) error {
	pf, err := s.load()
	if err != nil {
		return err
	}

	if _, ok := pf.Active[alias]; ok {
		delete(pf.Active, alias)
		return s.save(pf)
	}

	if _, ok := pf.Archived[alias]; ok {
		delete(pf.Archived, alias)
		return s.save(pf)
	}

	return fmt.Errorf("project %q not found", alias)
}

// Exists checks whether a project alias exists (in either active or archived).
func (s *Store) Exists(alias string) (bool, error) {
	pf, err := s.load()
	if err != nil {
		return false, err
	}
	_, active := pf.Active[alias]
	_, archived := pf.Archived[alias]
	return active || archived, nil
}
