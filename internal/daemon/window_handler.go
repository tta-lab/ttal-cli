package daemon

import (
	"fmt"
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// handleCloseWindow validates the window name and kills it with a short delay.
// The delay allows the HTTP response to reach the caller before the window (and
// the caller's process) is terminated.
func handleCloseWindow(req CloseWindowRequest) SendResponse {
	// Safety: only allow closing known reviewer window names.
	switch req.Window {
	case "review", "plan-review":
		// ok
	default:
		return SendResponse{OK: false, Error: fmt.Sprintf("refused to close unknown window %q", req.Window)}
	}

	if req.Session == "" {
		return SendResponse{OK: false, Error: "session is required"}
	}

	if !tmux.WindowExists(req.Session, req.Window) {
		// Window already gone — idempotent success.
		return SendResponse{OK: true}
	}

	go func() {
		time.Sleep(1 * time.Second)
		if err := tmux.KillWindow(req.Session, req.Window); err != nil {
			log.Printf("[daemon] close-window: failed to kill %s:%s: %v", req.Session, req.Window, err)
		} else {
			log.Printf("[daemon] close-window: killed %s:%s", req.Session, req.Window)
		}
	}()

	return SendResponse{OK: true}
}
