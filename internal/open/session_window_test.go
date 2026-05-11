//nolint:goconst // test clarity prefers inline strings over constants
package open

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

func TestSession_ManagerWindowFirst(t *testing.T) {
	origExport := exportTaskFn
	origWindowExists := windowExistsFn
	origAttachWindow := attachWindowFn
	origResolve := resolveTargetFn
	t.Cleanup(func() {
		exportTaskFn = origExport
		windowExistsFn = origWindowExists
		attachWindowFn = origAttachWindow
		resolveTargetFn = origResolve
	})

	var attachedSession, attachedWindow string
	var attachWindowCalled bool

	exportTaskFn = func(uuid string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: uuid, Owner: "astra", Tags: []string{"feature"}}, nil
	}
	windowExistsFn = func(session, window string) bool {
		return session == "ttal-default-astra" && window == "coder" //nolint:goconst
	}
	resolveTargetFn = func(task *taskwarrior.Task) (worker.TmuxTarget, error) {
		return worker.TmuxTarget{Session: "ttal-default-astra", Window: "coder", WorkDir: "/tmp/wt"}, nil //nolint:goconst
	}
	attachWindowFn = func(session, window string) error {
		attachWindowCalled = true
		attachedSession = session
		attachedWindow = window
		return nil
	}

	err := Session("aaaa0001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !attachWindowCalled {
		t.Error("expected attachWindowFn to be called for manager window")
	}
	if attachedSession != "ttal-default-astra" {
		t.Errorf("session = %q, want %q", attachedSession, "ttal-default-astra")
	}
	if attachedWindow != "coder" {
		t.Errorf("window = %q, want %q", attachedWindow, "coder")
	}
}

func TestSession_ManagerWindowFirst_NotFoundFallsBack(t *testing.T) {
	origExport := exportTaskFn
	origWindowExists := windowExistsFn
	origSessionExists := sessionExistsFn
	origAttach := attachFn
	origResolve := resolveTargetFn
	t.Cleanup(func() {
		exportTaskFn = origExport
		windowExistsFn = origWindowExists
		sessionExistsFn = origSessionExists
		attachFn = origAttach
		resolveTargetFn = origResolve
	})

	var attached string
	exportTaskFn = func(uuid string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: uuid, Owner: "astra", Tags: []string{"feature"}}, nil
	}
	windowExistsFn = func(session, window string) bool {
		return false
	}
	resolveTargetFn = func(task *taskwarrior.Task) (worker.TmuxTarget, error) {
		return worker.TmuxTarget{Session: "ttal-default-astra", Window: "coder", WorkDir: "/tmp/wt"}, nil
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
		t.Errorf("expected legacy session, got %q", attached)
	}
}

func TestSession_NoOwnerAndNoLegacy_ReturnsError(t *testing.T) {
	origExport := exportTaskFn
	origWindowExists := windowExistsFn
	origSessionExists := sessionExistsFn
	t.Cleanup(func() {
		exportTaskFn = origExport
		windowExistsFn = origWindowExists
		sessionExistsFn = origSessionExists
	})

	exportTaskFn = func(uuid string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: uuid, Tags: []string{"feature"}}, nil
	}
	windowExistsFn = func(session, window string) bool { return false }
	sessionExistsFn = func(name string) bool { return false }

	err := Session("aaaa0003")
	if err == nil {
		t.Fatal("expected error for task with no owner and no worker session")
	}
	if !strings.Contains(err.Error(), "no worker window") {
		t.Errorf("unexpected error: %v", err)
	}
}
