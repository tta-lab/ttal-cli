package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	checker := func(_, _, _ string) (bool, string, error) {
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
	checker := func(_, _, _ string) (bool, string, error) {
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
	checker := func(projectAlias, projectPath, prID string) (bool, string, error) {
		if projectAlias != "testproj" {
			return false, "", errors.New("unexpected projectAlias: " + projectAlias)
		}
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
	checker := func(_, _, _ string) (bool, string, error) {
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
	checker := func(_, _, prID string) (bool, string, error) {
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
	checker := func(_, _, _ string) (bool, string, error) { return false, "", nil }
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
		addedLgtmTag     string
		agentName        string
		allowedReviewers []string
		wantErr          bool
	}{
		{"plan-review-lead can add plan_lgtm", "plan_lgtm", "plan-review-lead", []string{"plan-review-lead"}, false},
		{"pr-review-lead can add implement_lgtm", "implement_lgtm", "pr-review-lead", []string{"pr-review-lead"}, false},
		{"coder cannot add plan_lgtm", "plan_lgtm", "coder", []string{"plan-review-lead"}, true},
		{"empty agent cannot add lgtm", "plan_lgtm", "", []string{"plan-review-lead"}, true},
		{"no lgtm tag added — not blocked", "", "coder", []string{"plan-review-lead"}, false},
		{"no pipeline (nil reviewers) rejects everyone", "plan_lgtm", "plan-review-lead", nil, true},
		{
			"multiple reviewers both allowed",
			"implement_lgtm", "pr-review-lead",
			[]string{"plan-review-lead", "pr-review-lead"}, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TTAL_AGENT_NAME", tt.agentName)
			err := checkLGTMGuard(tt.addedLgtmTag, tt.allowedReviewers)
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
	// Pipeline with no reviewer on the last stage (e.g. research/audit flows).
	const pipelinesNoReviewer = `
[hotfix]
tags = ["hotfix"]

[[hotfix.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
`
	tests := []struct {
		name     string
		toml     string
		taskTags []string
		wantErr  bool
	}{
		{"no pipeline match — allow", pipelinesBugfix, []string{"unrelated"}, false},
		{
			"last stage lgtm — allow",
			pipelinesBugfix,
			[]string{"bugfix", "fix", "fix_lgtm", "implement", "implement_lgtm"},
			false,
		},
		{"no last stage lgtm — block", pipelinesBugfix, []string{"bugfix"}, true},
		{"no pipeline config — allow", "", []string{"bugfix"}, false},
		{"first stage done but last not — block", pipelinesBugfix, []string{"bugfix", "fix", "fix_lgtm", "implement"}, true},
		// No-reviewer last stage: stage entry tag is sufficient for completion.
		{"no-reviewer last stage with entry tag — allow", pipelinesNoReviewer, []string{"hotfix", "implement"}, false},
		// No-reviewer last stage without entry tag — pipeline not reached yet, block.
		{"no-reviewer last stage without entry tag — block", pipelinesNoReviewer, []string{"hotfix"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate a worker context so the escape hatch doesn't bypass the gate.
			// No config.toml in temp dirs → isManagerRole returns false → gate enforced.
			t.Setenv("TTAL_AGENT_NAME", "some-worker")
			var dir string
			if tt.toml != "" {
				dir = writeTempPipelines(t, tt.toml)
			} else {
				dir = t.TempDir()
			}
			task := makeLGTMTask(tt.taskTags)
			err := checkPipelineDoneGuard(task, dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkPipelineDoneGuard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLgtmTagAdded(t *testing.T) {
	tests := []struct {
		name     string
		original []string
		modified []string
		want     string
	}{
		{"no lgtm tag", []string{"feature"}, []string{"feature"}, ""},
		{"plan_lgtm added", []string{"feature"}, []string{"feature", "plan_lgtm"}, "plan_lgtm"},
		{"plan_lgtm already present", []string{"plan_lgtm"}, []string{"plan_lgtm"}, ""},
		{"implement_lgtm added", []string{"plan_lgtm"}, []string{"plan_lgtm", "implement_lgtm"}, "implement_lgtm"},
		{"non-lgtm tag added", []string{}, []string{"urgent"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lgtmTagAdded(tt.original, tt.modified)
			if got != tt.want {
				t.Errorf("lgtmTagAdded() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckPipelineDoneGuard_MalformedTOML(t *testing.T) {
	// Malformed pipelines.toml — guard should fail-open (allow completion).
	// This documents the intent: corrupt config should not block task completion.
	t.Setenv("TTAL_AGENT_NAME", "some-worker")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte("not valid toml [[["), 0o644); err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}
	task := makeLGTMTask([]string{"bugfix"})
	if err := checkPipelineDoneGuard(task, dir); err != nil {
		t.Errorf("expected nil (fail-open) for malformed TOML, got: %v", err)
	}
}

func TestTagDiff(t *testing.T) {
	tests := []struct {
		name        string
		origTags    []string
		modTags     []string
		wantAdded   []string
		wantRemoved []string
	}{
		{"no changes", []string{"feature"}, []string{"feature"}, nil, nil},
		{"tag added", []string{"feature"}, []string{"feature", "urgent"}, []string{"urgent"}, nil},
		{"tag removed", []string{"feature", "urgent"}, []string{"feature"}, nil, []string{"urgent"}},
		{"added and removed", []string{"feature", "old"}, []string{"feature", "new"}, []string{"new"}, []string{"old"}},
		{"empty lists", []string{}, []string{}, nil, nil},
		// tagDiff uses sets: duplicates in orig resolve to same element, no false removal.
		{"duplicate in orig — no removal", []string{"a", "a"}, []string{"a"}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := tagDiff(tt.origTags, tt.modTags)
			if len(added) != len(tt.wantAdded) {
				t.Errorf("added = %v, want %v", added, tt.wantAdded)
			}
			for i, tag := range tt.wantAdded {
				if i >= len(added) || added[i] != tag {
					t.Errorf("added[%d] = %q, want %q", i, added[i], tag)
				}
			}
			if len(removed) != len(tt.wantRemoved) {
				t.Errorf("removed = %v, want %v", removed, tt.wantRemoved)
			}
			for i, tag := range tt.wantRemoved {
				if i >= len(removed) || removed[i] != tag {
					t.Errorf("removed[%d] = %q, want %q", i, removed[i], tag)
				}
			}
		})
	}
}

func makeTagTask(tags []string) hookTask {
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

func TestCheckTagGuard(t *testing.T) {
	// Set up a temp team dir with a manager and a worker.
	teamDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(teamDir, "yuki.md"), []byte("---\nrole: manager\n---\n"), 0o644); err != nil {
		t.Fatalf("write yuki.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamDir, "kestrel.md"), []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write kestrel.md: %v", err)
	}
	configDir := t.TempDir()
	configContent := fmt.Sprintf(`
default_team = "default"
[teams.default]
team_path = %q
`, teamDir)
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	tests := []struct {
		name      string
		agentName string
		origTags  []string
		modTags   []string
		wantErr   bool
	}{
		{"human adds tag", "", []string{"feature"}, []string{"feature", "newtag"}, false},
		{"manager adds tag", "yuki", []string{"feature"}, []string{"feature", "newtag"}, false},
		{"manager removes tag", "yuki", []string{"feature", "old"}, []string{"feature"}, false},
		{"worker adds non-lgtm tag", "kestrel", []string{"feature"}, []string{"feature", "urgent"}, true},
		{"worker removes tag", "kestrel", []string{"feature", "old"}, []string{"feature"}, true},
		// _lgtm additions are deferred to checkLGTMGuard, not blocked by checkTagGuard.
		{"worker adds _lgtm only", "kestrel", []string{"feature"}, []string{"feature", "plan_lgtm"}, false},
		{"worker no tag change", "kestrel", []string{"feature"}, []string{"feature"}, false},
		{"worker adds _lgtm and non-lgtm", "kestrel", []string{"feature"}, []string{"feature", "plan_lgtm", "urgent"}, true},
		{"unknown agent adds tag", "coder", []string{"feature"}, []string{"feature", "newtag"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TTAL_AGENT_NAME", tt.agentName)
			orig := makeTagTask(tt.origTags)
			mod := makeTagTask(tt.modTags)
			err := checkTagGuard(orig, mod, configDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkTagGuard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

func TestIsManagerRole(t *testing.T) {
	// Create a temp team dir with two agent .md files — one manager, one fixer.
	teamDir := t.TempDir()
	yukiContent := []byte("---\nrole: manager\n---\n# Yuki\n")
	if err := os.WriteFile(filepath.Join(teamDir, "yuki.md"), yukiContent, 0o644); err != nil {
		t.Fatalf("write yuki.md: %v", err)
	}
	kestrelContent := []byte("---\nrole: fixer\n---\n# Kestrel\n")
	if err := os.WriteFile(filepath.Join(teamDir, "kestrel.md"), kestrelContent, 0o644); err != nil {
		t.Fatalf("write kestrel.md: %v", err)
	}

	// Create a minimal config.toml pointing to the temp team dir.
	configDir := t.TempDir()
	configContent := fmt.Sprintf(`
default_team = "default"
[teams.default]
team_path = %q
`, teamDir)
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	tests := []struct {
		name      string
		agentName string
		want      bool
	}{
		{"manager role bypasses", "yuki", true},
		{"fixer role does not bypass", "kestrel", false},
		{"unknown agent does not bypass", "coder", false},
		{"empty name does not bypass", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isManagerRole(tt.agentName, configDir)
			if got != tt.want {
				t.Errorf("isManagerRole(%q) = %v, want %v", tt.agentName, got, tt.want)
			}
		})
	}
}

func TestIsManagerRole_NoConfig(t *testing.T) {
	// No config.toml — should fail-safe (return false).
	emptyDir := t.TempDir()
	if got := isManagerRole("yuki", emptyDir); got {
		t.Error("expected false when config.toml is missing")
	}
}

func TestCheckPipelineDoneGuard_EscapeHatch(t *testing.T) {
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
	// Set up a team dir with a manager and a fixer.
	teamDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(teamDir, "yuki.md"), []byte("---\nrole: manager\n---\n"), 0o644); err != nil {
		t.Fatalf("write yuki.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamDir, "kestrel.md"), []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write kestrel.md: %v", err)
	}

	// Config dir with pipelines.toml AND config.toml pointing to team dir.
	configDir := writeTempPipelines(t, pipelinesBugfix)
	configContent := fmt.Sprintf(`
default_team = "default"
[teams.default]
team_path = %q
`, teamDir)
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Task that would normally be blocked (has pipeline tag, no last-stage lgtm).
	task := makeLGTMTask([]string{"bugfix"})

	t.Run("human bypasses gate", func(t *testing.T) {
		t.Setenv("TTAL_AGENT_NAME", "")
		if err := checkPipelineDoneGuard(task, configDir); err != nil {
			t.Errorf("expected nil for human, got: %v", err)
		}
	})

	t.Run("manager role bypasses gate", func(t *testing.T) {
		t.Setenv("TTAL_AGENT_NAME", "yuki")
		if err := checkPipelineDoneGuard(task, configDir); err != nil {
			t.Errorf("expected nil for manager role, got: %v", err)
		}
	})

	t.Run("fixer role is still gated", func(t *testing.T) {
		t.Setenv("TTAL_AGENT_NAME", "kestrel")
		if err := checkPipelineDoneGuard(task, configDir); err == nil {
			t.Error("expected error for fixer role, got nil")
		}
	})

	t.Run("unknown agent (worker) is still gated", func(t *testing.T) {
		t.Setenv("TTAL_AGENT_NAME", "coder")
		if err := checkPipelineDoneGuard(task, configDir); err == nil {
			t.Error("expected error for unknown agent, got nil")
		}
	})
}
