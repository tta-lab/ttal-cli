package worker

import (
	"errors"
	"strings"
	"testing"
)

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
	task := makeTask("7:lgtm", "testproj")
	resolver := mockResolver(map[string]string{"testproj": "/some/project"})
	checker := func(projectPath, prID string) (bool, string, error) {
		if prID != "7:lgtm" {
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
	task := makeTask("7:lgtm", "testproj")
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
