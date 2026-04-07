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

func TestActiveTasksByOwner_NoWorkerStageTasks(t *testing.T) {
	cfg, err := Load(writeTempTOML(t, ownershipTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	orig := exportTasksByFilterFn
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	exportTasksByFilterFn = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "11111111-0000-0000-0000-000000000001", Tags: []string{"feature", "design"}},
			{UUID: "11111111-0000-0000-0000-000000000002", Tags: []string{"bugfix", "fix"}},
		}, nil
	}

	got, err := ActiveTasksByOwner(cfg, "inke")
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

	orig := exportTasksByFilterFn
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	exportTasksByFilterFn = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "22222222-0000-0000-0000-000000000001", Tags: []string{"feature", "implement"}},
			{UUID: "22222222-0000-0000-0000-000000000002", Tags: []string{"bugfix", "review"}},
		}, nil
	}

	got, err := ActiveTasksByOwner(cfg, "inke")
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

	orig := exportTasksByFilterFn
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	exportTasksByFilterFn = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "33333333-0000-0000-0000-000000000001", Tags: []string{"feature", "design"}},
			{UUID: "33333333-0000-0000-0000-000000000002", Tags: []string{"feature", "implement"}},
		}, nil
	}

	got, err := ActiveTasksByOwner(cfg, "inke")
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

	orig := exportTasksByFilterFn
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	exportTasksByFilterFn = func(args ...string) ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{
			{UUID: "44444444-0000-0000-0000-000000000001", Tags: []string{"random-tag"}},
		}, nil
	}

	got, err := ActiveTasksByOwner(cfg, "inke")
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

	orig := exportTasksByFilterFn
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	exportTasksByFilterFn = func(args ...string) ([]taskwarrior.Task, error) {
		return nil, errTaskwarriorUnavailable
	}

	_, err = ActiveTasksByOwner(cfg, "inke")
	if err == nil {
		t.Error("expected error from taskwarrior, got nil")
	}
}
