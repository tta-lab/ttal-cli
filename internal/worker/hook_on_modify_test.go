package worker

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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
		name             string
		original         []string
		modified         []string
		agentName        string
		allowedReviewers []string
		wantErr          bool
	}{
		{
			name:             "plan-review-lead can add lgtm when listed as reviewer",
			modified:         []string{"lgtm"},
			agentName:        "plan-review-lead",
			allowedReviewers: []string{"plan-review-lead"},
			wantErr:          false,
		},
		{
			name:             "pr-review-lead can add lgtm when listed as reviewer",
			modified:         []string{"lgtm"},
			agentName:        "pr-review-lead",
			allowedReviewers: []string{"pr-review-lead"},
			wantErr:          false,
		},
		{
			name:             "coder cannot add lgtm",
			modified:         []string{"lgtm"},
			agentName:        "coder",
			allowedReviewers: []string{"plan-review-lead"},
			wantErr:          true,
		},
		{
			name:             "empty agent cannot add lgtm",
			modified:         []string{"lgtm"},
			agentName:        "",
			allowedReviewers: []string{"plan-review-lead"},
			wantErr:          true,
		},
		{
			name:             "lgtm already present is not blocked",
			original:         []string{"lgtm"},
			modified:         []string{"lgtm"},
			agentName:        "coder",
			allowedReviewers: []string{"plan-review-lead"},
			wantErr:          false,
		},
		{
			name:             "no pipeline (nil reviewers) rejects everyone",
			modified:         []string{"lgtm"},
			agentName:        "plan-review-lead",
			allowedReviewers: nil,
			wantErr:          true,
		},
		{
			name:             "unrelated tag change not blocked",
			modified:         []string{"urgent"},
			agentName:        "coder",
			allowedReviewers: []string{"plan-review-lead"},
			wantErr:          false,
		},
		{
			name:             "multiple reviewers across stages both allowed",
			modified:         []string{"lgtm"},
			agentName:        "pr-review-lead",
			allowedReviewers: []string{"plan-review-lead", "pr-review-lead"},
			wantErr:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TTAL_AGENT_NAME", tt.agentName)
			lgtmAdded := !slices.Contains(tt.original, "lgtm") && slices.Contains(tt.modified, "lgtm")
			err := checkLGTMGuard(lgtmAdded, tt.allowedReviewers)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkLGTMGuard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPipelineDoneGuard(t *testing.T) {
	const pipelinesBugfix = `
[bugfix]
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "worker"
gate = "auto"
reviewer = "pr-review-lead"

[[bugfix.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
reviewer = "pr-review-lead"
`
	tests := []struct {
		name     string
		toml     string
		taskTags []string
		wantErr  bool
	}{
		{
			name:     "no pipeline match — allow",
			toml:     pipelinesBugfix,
			taskTags: []string{"unrelated"},
			wantErr:  false,
		},
		{
			name:     "pipeline match + pipeline_done — allow",
			toml:     pipelinesBugfix,
			taskTags: []string{"bugfix", "pipeline_done"},
			wantErr:  false,
		},
		{
			name:     "pipeline match + no pipeline_done — block",
			toml:     pipelinesBugfix,
			taskTags: []string{"bugfix"},
			wantErr:  true,
		},
		{
			name:     "no pipeline config — allow",
			toml:     "", // empty dir, no pipelines.toml
			taskTags: []string{"bugfix"},
			wantErr:  false,
		},
		{
			name:     "multi-stage pipeline no pipeline_done — block",
			toml:     pipelinesBugfix,
			taskTags: []string{"bugfix", "worker"},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string
			if tt.toml != "" {
				dir = writeTempPipelines(t, tt.toml)
			} else {
				dir = t.TempDir() // no pipelines.toml
			}
			task := makeLGTMTask(tt.taskTags)
			err := checkPipelineDoneGuard(task, dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkPipelineDoneGuard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPipelineDoneGuard_MalformedTOML(t *testing.T) {
	// Malformed pipelines.toml — guard should fail-open (allow completion).
	// This documents the intent: corrupt config should not block task completion.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte("not valid toml [[["), 0o644); err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}
	task := makeLGTMTask([]string{"bugfix"})
	if err := checkPipelineDoneGuard(task, dir); err != nil {
		t.Errorf("expected nil (fail-open) for malformed TOML, got: %v", err)
	}
}

func writeTempPipelines(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}
	return dir
}

func TestResolveAllowedReviewers(t *testing.T) {
	const pipelinesWithReviewers = `
[standard]
tags = ["feature"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-review-lead"

[[standard.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
reviewer = "pr-review-lead"
`
	const pipelinesNoReviewer = `
[hotfix]
tags = ["hotfix"]

[[hotfix.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
`

	tests := []struct {
		name       string
		toml       string
		taskTags   []string
		wantResult []string
	}{
		{
			name:       "matched pipeline collects all stage reviewers",
			toml:       pipelinesWithReviewers,
			taskTags:   []string{"feature"},
			wantResult: []string{"plan-review-lead", "pr-review-lead"},
		},
		{
			name:       "no pipeline match returns nil",
			toml:       pipelinesWithReviewers,
			taskTags:   []string{"unrelated"},
			wantResult: nil,
		},
		{
			name:       "pipeline with no reviewer fields returns nil",
			toml:       pipelinesNoReviewer,
			taskTags:   []string{"hotfix"},
			wantResult: nil,
		},
		{
			name:       "missing pipelines.toml returns nil",
			toml:       "", // empty dir — no file written
			taskTags:   []string{"feature"},
			wantResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string
			if tt.toml != "" {
				dir = writeTempPipelines(t, tt.toml)
			} else {
				dir = t.TempDir() // no pipelines.toml
			}
			task := makeLGTMTask(tt.taskTags)
			got := resolveAllowedReviewers(task, dir)
			if len(got) != len(tt.wantResult) {
				t.Fatalf("resolveAllowedReviewers() = %v, want %v", got, tt.wantResult)
			}
			for i, r := range tt.wantResult {
				if got[i] != r {
					t.Errorf("resolveAllowedReviewers()[%d] = %q, want %q", i, got[i], r)
				}
			}
		})
	}
}
