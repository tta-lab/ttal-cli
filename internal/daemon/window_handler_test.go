package daemon

import "testing"

func TestHandleCloseWindow_RejectsUnknownWindow(t *testing.T) {
	resp := handleCloseWindow(CloseWindowRequest{Session: "s", Window: "unknown"})
	if resp.OK {
		t.Error("expected rejection for unknown window name")
	}
}

func TestHandleCloseWindow_RequiresSession(t *testing.T) {
	resp := handleCloseWindow(CloseWindowRequest{Session: "", Window: "review"})
	if resp.OK {
		t.Error("expected rejection for empty session")
	}
}

func TestHandleCloseWindow_AcceptsReviewWindows(t *testing.T) {
	// In test env without tmux, WindowExists returns false → idempotent success path.
	for _, window := range []string{"review", "plan-review"} {
		resp := handleCloseWindow(CloseWindowRequest{Session: "test", Window: window})
		if !resp.OK {
			t.Errorf("expected OK for window %q, got error: %s", window, resp.Error)
		}
	}
}
