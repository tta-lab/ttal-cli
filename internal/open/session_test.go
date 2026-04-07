package open

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

type stubConfig struct{ teamName string }

func (c *stubConfig) TeamName() string { return c.teamName }

func TestSession_OwnerFallback_AttachesOwnerSession(t *testing.T) {
	origExport := exportTaskFn
	origExists := sessionExistsFn
	origAttach := attachFn
	origLoader := configLoaderFn
	t.Cleanup(func() {
		exportTaskFn = origExport
		sessionExistsFn = origExists
		attachFn = origAttach
		configLoaderFn = origLoader
	})

	var attached string
	exportTaskFn = func(uuid string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: uuid, Owner: "astra", Tags: []string{"feature"}}, nil
	}
	sessionExistsFn = func(name string) bool {
		return name == "ttal-testteam-astra"
	}
	attachFn = func(name string) error {
		attached = name
		return nil
	}
	configLoaderFn = func() (configWithTeamName, error) {
		return &stubConfig{teamName: "testteam"}, nil
	}

	err := Session("aaaa0001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attached != "ttal-testteam-astra" {
		t.Errorf("expected owner session ttal-testteam-astra, got %q", attached)
	}
}

func TestSession_WorkerSessionExists_AttachesWorker(t *testing.T) {
	origExport := exportTaskFn
	origExists := sessionExistsFn
	origAttach := attachFn
	t.Cleanup(func() {
		exportTaskFn = origExport
		sessionExistsFn = origExists
		attachFn = origAttach
	})

	var attached string
	exportTaskFn = func(uuid string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: uuid, Owner: "astra", Tags: []string{"feature"}}, nil
	}
	sessionExistsFn = func(name string) bool {
		return name == "w-aaaa0002"
	}
	attachFn = func(name string) error {
		attached = name
		return nil
	}

	err := Session("aaaa0002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attached != "w-aaaa0002" {
		t.Errorf("expected worker session, got %q", attached)
	}
}

func TestSession_NoOwner_ReturnsNoSessionError(t *testing.T) {
	origExport := exportTaskFn
	origExists := sessionExistsFn
	t.Cleanup(func() {
		exportTaskFn = origExport
		sessionExistsFn = origExists
	})

	exportTaskFn = func(uuid string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: uuid, Tags: []string{"feature"}}, nil
	}
	sessionExistsFn = func(name string) bool { return false }

	err := Session("aaaa0003")
	if err == nil {
		t.Fatal("expected error for task with no owner and no worker session")
	}
	if !strings.Contains(err.Error(), "no worker or agent session") {
		t.Errorf("unexpected error: %v", err)
	}
}
