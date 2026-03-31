package daemon

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
)

// handleSubagent returns an HTTP handler that runs a subagent loop server-side
// and streams NDJSON events back to the client.
func handleSubagent(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ask.SubagentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid subagent JSON: " + err.Error()})
			return
		}

		if req.Name == "" {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "name is required"})
			return
		}
		if req.Prompt == "" {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "prompt is required"})
			return
		}

		// Require flushing support — streaming is the whole point of this endpoint.
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeHTTPJSON(w, http.StatusInternalServerError,
				SendResponse{OK: false, Error: "streaming not supported by response writer"})
			return
		}

		// Set up NDJSON streaming response
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)

		// Cancel the agent loop if the client disconnects mid-stream.
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		emit := func(event ask.Event) {
			if err := enc.Encode(event); err != nil {
				log.Printf("[daemon] subagent: write event error: %v", err)
				cancel() // stop the agent loop — client is gone
				return
			}
			flusher.Flush()
		}

		if err := ask.RunSubagent(ctx, req, cfg, emit); err != nil {
			// RunSubagent already emits error events for agent errors.
			// This catches infrastructure errors (temenos unreachable, etc.)
			emit(ask.Event{Type: ask.EventError, Message: err.Error()})
		}
	}
}
