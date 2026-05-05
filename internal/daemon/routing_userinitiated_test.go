package daemon

import (
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/sendfmt"
)

func TestDispatchSystemSend_UserInitiatedTrue_WrapsAgentDelivery(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	setupAgentDirs(t, tmp, "kestrel")
	cfg := &config.Config{TeamPath: tmp}

	restore := sendfmt.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 5, 14, 32, 5, 0, time.UTC)
	})
	t.Cleanup(func() { restore() })

	captured, cleanup := setupAgentDeliverToAgentCapture()
	t.Cleanup(cleanup)

	req := SendRequest{From: "system", To: "kestrel", Message: "hello", UserInitiated: true}
	if err := dispatchSystemSend(cfg, nil, nil, nil, req); err != nil {
		t.Fatalf("dispatchSystemSend: %v", err)
	}
	want := "[14:32:05] hello"
	if captured.msg != want {
		t.Errorf("captured = %q, want %q", captured.msg, want)
	}
}

func TestDispatchSystemSend_UserInitiatedFalse_DeliversBare(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	setupAgentDirs(t, tmp, "kestrel")
	cfg := &config.Config{TeamPath: tmp}

	captured, cleanup := setupAgentDeliverToAgentCapture()
	t.Cleanup(cleanup)

	req := SendRequest{From: "system", To: "kestrel", Message: "run skill get breathe"}
	if err := dispatchSystemSend(cfg, nil, nil, nil, req); err != nil {
		t.Fatalf("dispatchSystemSend: %v", err)
	}
	if captured.msg != "run skill get breathe" {
		t.Errorf("captured = %q, want raw passthrough", captured.msg)
	}
}
