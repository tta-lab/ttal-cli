package daemon

import (
	"fmt"
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// handleCloseWindow validates the window name and kills it with a short delay.
// The delay allows the HTTP response to reach the caller before the window (and
// the caller's process) is terminated.
func handleCloseWindow(req CloseWindowRequest) SendResponse {
	return handleCloseWindowWithConfigDir(req, config.DefaultConfigDir())
}

func handleCloseWindowWithConfigDir(req CloseWindowRequest, configDir string) SendResponse {
	// Safety: only allow closing known reviewer windows (from pipelines.toml).
	if !isKnownReviewerWindow(req.Window, configDir) {
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

// isKnownReviewerWindow returns true if name is configured as a reviewer in any pipeline stage.
func isKnownReviewerWindow(name, configDir string) bool {
	pipelineCfg, err := pipeline.Load(configDir)
	if err != nil {
		log.Printf("[daemon] close-window: could not load pipelines — refusing %q", name)
		return false
	}
	return pipelineCfg.HasReviewer(name)
}
