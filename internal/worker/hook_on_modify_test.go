package worker

import (
	"errors"
	"strings"
	"testing"
)

// makeTask builds a minimal hookTask with the given fields.
func makeTask(prID, projectPath string) hookTask {
	t := hookTask{}
	if prID != "" {
		t["pr_id"] = prID
	}
	if projectPath != "" {
		t["project_path"] = projectPath
	}
	return t
}

func TestValidateTaskCompletion_NoPRID(t *testing.T) {
	task := makeTask("", "")
	// No pr_id — should allow completion immediately without calling checker.
	checkerCalled := false
	checker := func(_, _ string) (bool, error) {
		checkerCalled = true
		return false, nil
	}
	if err := validateTaskCompletion(task, checker); err != nil {
		t.Errorf("expected nil error for task with no pr_id, got: %v", err)
	}
	if checkerCalled {
		t.Error("checker should not be called when pr_id is empty")
	}
}

func TestValidateTaskCompletion_PRIDButNoProjectPath(t *testing.T) {
	task := makeTask("42", "")
	// Has pr_id but no project_path — should return an error before calling checker.
	checkerCalled := false
	checker := func(_, _ string) (bool, error) {
		checkerCalled = true
		return false, nil
	}
	err := validateTaskCompletion(task, checker)
	if err == nil {
		t.Fatal("expected error when pr_id is set but project_path is empty")
	}
	if checkerCalled {
		t.Error("checker should not be called when project_path is missing")
	}
}

func TestValidateTaskCompletion_PRMerged(t *testing.T) {
	task := makeTask("7", "/some/project")
	checker := func(projectPath, prID string) (bool, error) {
		if projectPath != "/some/project" {
			return false, errors.New("unexpected projectPath: " + projectPath)
		}
		if prID != "7" {
			return false, errors.New("unexpected prID: " + prID)
		}
		return true, nil // merged
	}
	if err := validateTaskCompletion(task, checker); err != nil {
		t.Errorf("expected nil error for merged PR, got: %v", err)
	}
}

func TestValidateTaskCompletion_PROpen(t *testing.T) {
	task := makeTask("7", "/some/project")
	checker := func(_, _ string) (bool, error) {
		return false, nil // not merged
	}
	err := validateTaskCompletion(task, checker)
	if err == nil {
		t.Fatal("expected error for unmerged PR")
	}
}

func TestValidateTaskCompletion_PRMergedWithLGTM(t *testing.T) {
	task := makeTask("7:lgtm", "/some/project")
	checker := func(projectPath, prID string) (bool, error) {
		if prID != "7:lgtm" {
			return false, errors.New("unexpected prID: " + prID)
		}
		return true, nil
	}
	if err := validateTaskCompletion(task, checker); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateTaskCompletion_PROpenWithLGTM(t *testing.T) {
	task := makeTask("7:lgtm", "/some/project")
	checker := func(_, _ string) (bool, error) { return false, nil }
	err := validateTaskCompletion(task, checker)
	if err == nil {
		t.Fatal("expected error for unmerged PR")
	}
	if !strings.Contains(err.Error(), "#7") {
		t.Errorf("expected error to contain '#7', got: %v", err)
	}
}
