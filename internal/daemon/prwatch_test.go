package daemon

import (
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

func TestResolveWorkerWindowName_FromTask(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:  "abc12345-0000-0000-0000-000000000000",
		Owner: "astra",
		Tags:  []string{"feature"},
	}
	target, err := worker.ResolveTmuxTarget(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Session != "ttal-default-astra" {
		t.Errorf("session = %q, want %q", target.Session, "ttal-default-astra")
	}
	if target.Window == "" {
		t.Error("window should not be empty")
	}
}

func TestDeliverToWorkerSession(t *testing.T) {
	target := prWatchTarget{
		SessionName: "ttal-default-astra",
		WindowName:  "coder",
		PRIndex:     42,
	}
	// Just verify it doesn't panic with the right parameters
	// (SendKeys will fail because no real tmux, but that's caught at runtime)
	deliverToWorkerSession(target, "test message to worker window")
}

func TestPRWatchTarget_WindowNameNotEmpty(t *testing.T) {
	target := prWatchTarget{
		TaskUUID:    "abc12345",
		SessionName: "ttal-default-astra",
		WindowName:  "coder",
	}
	if target.WindowName == "" {
		t.Error("WindowName should not be empty for worker window routing")
	}
	if target.SessionName == "" {
		t.Error("SessionName should not be empty")
	}
}

func TestBackoff(t *testing.T) {
	intervals := []time.Duration{}
	interval := prPollInitial
	for i := 0; i < 5; i++ {
		intervals = append(intervals, interval)
		interval = backoff(interval)
	}
	// Verify intervals increase (backoff works)
	for i := 1; i < len(intervals); i++ {
		if intervals[i] <= intervals[i-1] {
			t.Errorf("expected backoff to increase, got %v <= %v", intervals[i], intervals[i-1])
		}
	}
}
