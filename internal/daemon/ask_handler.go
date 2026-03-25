package daemon

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
)

// handleAsk returns an HTTP handler that runs the ask agent loop server-side
// and streams NDJSON events back to the client.
func handleAsk(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ask.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid ask JSON: " + err.Error()})
			return
		}

		// Validate mode
		switch req.Mode {
		case ask.ModeProject, ask.ModeRepo, ask.ModeURL, ask.ModeWeb, ask.ModeGeneral:
			// valid
		default:
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid mode: " + string(req.Mode)})
			return
		}

		// Set up NDJSON streaming response
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Printf("[daemon] ask: response writer does not support flushing")
		}

		enc := json.NewEncoder(w)

		emit := func(event ask.Event) {
			if err := enc.Encode(event); err != nil {
				log.Printf("[daemon] ask: write event error: %v", err)
				return
			}
			if ok {
				flusher.Flush()
			}
		}

		if err := ask.RunAsk(r.Context(), req, cfg, emit); err != nil {
			// RunAsk already emits error events for agent errors.
			// This catches infrastructure errors (temenos unreachable, etc.)
			emit(ask.Event{Type: ask.EventError, Message: err.Error()})
		}
	}
}
