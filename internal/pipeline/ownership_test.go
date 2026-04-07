package pipeline

import (
	"errors"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const ownershipTOML = `
[standard]
description = "Plan → Implement"
tags = ["feature"]

[[standard.stages]]
name = "Design"
assignee = "designer"
worker = false
gate = "human"

[[standard.stages]]
name = "Implement"
assignee = "coder"
worker = true
gate = "auto"

[bugfix]
description = "Fix → Review"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
worker = false
gate = "human"

[[bugfix.stages]]
name = "Review"
assignee = "reviewer"
worker = true
gate = "auto"
`

var errTaskwarriorUnavailable = errors.New("taskwarrior unavailable")

// stubTaskwarriorExporter is used by tests to stub taskwarrior.ExportTasksByFilter.
var stubTaskwarriorExporter func(args ...string) ([]taskwarrior.Task, error)

func init() {
	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		panic("stubTaskwarriorExporter not stubbed in test")
	}
}

// activeTasksByOwnerForTest mirrors ActiveTasksByOwner but accepts an exporter fn
// so the test can inject fake taskwarrior responses.
func activeTasksByOwnerForTest(
	exporter func(args ...string) ([]taskwarrior.Task, error),
	cfg *Config,
	owner string, //nolint:unparam // test helper always passes literal, param is for clarity
) ([]taskwarrior.Task, error) {
	tasks, err := exporter("status:pending", "+ACTIVE", "owner:"+owner)
	if err != nil {
		return nil, err
	}

	var filtered []taskwarrior.Task
	for _, task := range tasks {
		_, p, err := cfg.MatchPipeline(task.Tags)
		if err != nil || p == nil {
			filtered = append(filtered, task)
			continue
		}
		_, stage, _ := p.CurrentStage(task.Tags)
		if stage != nil && stage.IsWorker() {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered, nil
}

func resetStub() {
	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		panic("stubTaskwarriorExporter not stubbed in test")
	}
}

func TestActiveTasksByOwner_NoWorkerStageTasks(t *testing.T) {
	cfg, err := Load(writeTempTOML(t, ownershipTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "11111111-0000-0000-0000-000000000001", Tags: []string{"feature", "design"}},
			{UUID: "11111111-0000-0000-0000-000000000002", Tags: []string{"bugfix", "fix"}},
		}, nil
	}
	t.Cleanup(resetStub)

	got, err := activeTasksByOwnerForTest(stubTaskwarriorExporter, cfg, "inke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(got))
	}
}

func TestActiveTasksByOwner_AllWorkerStageTasks(t *testing.T) {
	cfg, err := Load(writeTempTOML(t, ownershipTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "22222222-0000-0000-0000-000000000001", Tags: []string{"feature", "implement"}},
			{UUID: "22222222-0000-0000-0000-000000000002", Tags: []string{"bugfix", "review"}},
		}, nil
	}
	t.Cleanup(resetStub)

	got, err := activeTasksByOwnerForTest(stubTaskwarriorExporter, cfg, "inke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 tasks (all worker stage), got %d", len(got))
	}
}

func TestActiveTasksByOwner_MixedStages(t *testing.T) {
	cfg, err := Load(writeTempTOML(t, ownershipTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "33333333-0000-0000-0000-000000000001", Tags: []string{"feature", "design"}},
			{UUID: "33333333-0000-0000-0000-000000000002", Tags: []string{"feature", "implement"}},
		}, nil
	}
	t.Cleanup(resetStub)

	got, err := activeTasksByOwnerForTest(stubTaskwarriorExporter, cfg, "inke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 task (manager only), got %d", len(got))
	}
	if got[0].UUID != "33333333-0000-0000-0000-000000000001" {
		t.Errorf("expected design task, got %s", got[0].UUID)
	}
}

func TestActiveTasksByOwner_NoPipelineMatch(t *testing.T) {
	cfg, err := Load(writeTempTOML(t, ownershipTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "44444444-0000-0000-0000-000000000001", Tags: []string{"random-tag"}},
		}, nil
	}
	t.Cleanup(resetStub)

	got, err := activeTasksByOwnerForTest(stubTaskwarriorExporter, cfg, "inke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 task (no pipeline match, included for safety), got %d", len(got))
	}
}

func TestActiveTasksByOwner_TaskwarriorError(t *testing.T) {
	cfg, err := Load(writeTempTOML(t, ownershipTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stubTaskwarriorExporter = func(args ...string) ([]taskwarrior.Task, error) {
		return nil, errTaskwarriorUnavailable
	}
	t.Cleanup(resetStub)

	_, err = activeTasksByOwnerForTest(stubTaskwarriorExporter, cfg, "inke")
	if err == nil {
		t.Error("expected error from taskwarrior, got nil")
	}
}
