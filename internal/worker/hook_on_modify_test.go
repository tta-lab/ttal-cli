package worker

import (
	"errors"
	"strings"
	"testing"
)

// makeEnrichTask builds a minimal hookTask for enrichInline tests.
func makeEnrichTask(project, description string) hookTask {
	t := hookTask{}
	if project != "" {
		t["project"] = project
	}
	if description != "" {
		t["description"] = description
	}
	t["uuid"] = "test-uuid-1234"
	return t
}

func TestEnrichInline_EmptyProject(t *testing.T) {
	task := makeEnrichTask("", "add feature")
	if err := enrichInline(task, nil); err != nil {
		t.Errorf("expected nil for empty project, got: %v", err)
	}
}

func TestEnrichInline_RegisteredProject(t *testing.T) {
	task := makeEnrichTask("testproj", "add new feature for testing")
	resolver := mockResolver(map[string]string{"testproj": "/some/project"})
	if err := enrichInline(task, resolver); err != nil {
		t.Errorf("expected nil for registered project, got: %v", err)
	}
	// Branch is no longer set by enrichInline — it's computed at runtime.
	if task["branch"] != nil {
		t.Error("branch should not be set by enrichInline (computed at runtime)")
	}
}

func TestEnrichInline_UnregisteredProject(t *testing.T) {
	task := makeEnrichTask("nonexistent", "add feature")
	resolver := mockResolver(map[string]string{}) // empty — no projects
	err := enrichInline(task, resolver)
	if err == nil {
		t.Fatal("expected error for unregistered project")
	}
	if task["branch"] != nil {
		t.Error("branch should not be set when project is unregistered")
	}
}

// makeTask builds a minimal hookTask with the given fields.
func makeTask(prID, projectAlias string) hookTask {
	t := hookTask{}
	if prID != "" {
		t["pr_id"] = prID
	}
	if projectAlias != "" {
		t["project"] = projectAlias
	}
	return t
}

// mockResolver returns a resolver that maps project aliases to paths.
func mockResolver(mapping map[string]string) pathResolver {
	return func(name string) string {
		return mapping[name]
	}
}

func TestValidateTaskCompletion_NoPRID(t *testing.T) {
	task := makeTask("", "")
	// No pr_id — should allow completion immediately without calling checker.
	checkerCalled := false
	checker := func(_, _ string) (bool, string, error) {
		checkerCalled = true
		return false, "", nil
	}
	if _, err := validateTaskCompletion(task, checker, nil); err != nil {
		t.Errorf("expected nil error for task with no pr_id, got: %v", err)
	}
	if checkerCalled {
		t.Error("checker should not be called when pr_id is empty")
	}
}

func TestValidateTaskCompletion_PRIDButNoProject(t *testing.T) {
	task := makeTask("42", "")
	// Has pr_id but no project — should return an error before calling checker.
	checkerCalled := false
	checker := func(_, _ string) (bool, string, error) {
		checkerCalled = true
		return false, "", nil
	}
	resolver := mockResolver(map[string]string{})
	_, err := validateTaskCompletion(task, checker, resolver)
	if err == nil {
		t.Fatal("expected error when pr_id is set but project is empty")
	}
	if checkerCalled {
		t.Error("checker should not be called when project is missing")
	}
}

func TestValidateTaskCompletion_PRMerged(t *testing.T) {
	task := makeTask("7", "testproj")
	resolver := mockResolver(map[string]string{"testproj": "/some/project"})
	checker := func(projectPath, prID string) (bool, string, error) {
		if projectPath != "/some/project" {
			return false, "", errors.New("unexpected projectPath: " + projectPath)
		}
		if prID != "7" {
			return false, "", errors.New("unexpected prID: " + prID)
		}
		return true, "feat: test PR title", nil // merged
	}
	prTitle, err := validateTaskCompletion(task, checker, resolver)
	if err != nil {
		t.Errorf("expected nil error for merged PR, got: %v", err)
	}
	if prTitle != "feat: test PR title" {
		t.Errorf("expected PR title %q, got %q", "feat: test PR title", prTitle)
	}
}

func TestValidateTaskCompletion_PROpen(t *testing.T) {
	task := makeTask("7", "testproj")
	resolver := mockResolver(map[string]string{"testproj": "/some/project"})
	checker := func(_, _ string) (bool, string, error) {
		return false, "", nil // not merged
	}
	_, err := validateTaskCompletion(task, checker, resolver)
	if err == nil {
		t.Fatal("expected error for unmerged PR")
	}
}

func TestValidateTaskCompletion_PRMergedWithLGTM(t *testing.T) {
	task := makeTask("7", "testproj")
	resolver := mockResolver(map[string]string{"testproj": "/some/project"})
	checker := func(projectPath, prID string) (bool, string, error) {
		if prID != "7" {
			return false, "", errors.New("unexpected prID: " + prID)
		}
		return true, "fix: lgtm title", nil
	}
	prTitle, err := validateTaskCompletion(task, checker, resolver)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if prTitle != "fix: lgtm title" {
		t.Errorf("expected PR title %q, got %q", "fix: lgtm title", prTitle)
	}
}

func TestValidateTaskCompletion_PROpenWithLGTM(t *testing.T) {
	task := makeTask("7", "testproj")
	resolver := mockResolver(map[string]string{"testproj": "/some/project"})
	checker := func(_, _ string) (bool, string, error) { return false, "", nil }
	_, err := validateTaskCompletion(task, checker, resolver)
	if err == nil {
		t.Fatal("expected error for unmerged PR")
	}
	if !strings.Contains(err.Error(), "#7") {
		t.Errorf("expected error to contain '#7', got: %v", err)
	}
}

func makeLGTMTask(tags []string) hookTask {
	t := hookTask{}
	t["uuid"] = "test-uuid"
	t["status"] = "pending"
	if len(tags) > 0 {
		tagSlice := make([]interface{}, len(tags))
		for i, tag := range tags {
			tagSlice[i] = tag
		}
		t["tags"] = tagSlice
	}
	return t
}

func TestCheckLGTMGuard(t *testing.T) {
	tests := []struct {
		name     string
		original []string
		modified []string
		role     string
		wantErr  bool
	}{
		{
			name:     "reviewer can add lgtm",
			original: nil,
			modified: []string{"lgtm"},
			role:     "reviewer",
			wantErr:  false,
		},
		{
			name:     "coder cannot add lgtm",
			original: nil,
			modified: []string{"lgtm"},
			role:     "coder",
			wantErr:  true,
		},
		{
			name:     "empty role cannot add lgtm",
			original: nil,
			modified: []string{"lgtm"},
			role:     "",
			wantErr:  true,
		},
		{
			name:     "lgtm already present is not blocked",
			original: []string{"lgtm"},
			modified: []string{"lgtm"},
			role:     "coder",
			wantErr:  false,
		},
		{
			name:     "unrelated tag change not blocked",
			original: nil,
			modified: []string{"urgent"},
			role:     "coder",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TTAL_AGENT_NAME", tt.role)
			orig := makeLGTMTask(tt.original)
			mod := makeLGTMTask(tt.modified)
			err := checkLGTMGuard(orig, mod)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
