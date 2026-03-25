package daemon

import (
	"context"
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

		// Validate mode using the canonical allowlist.
		if !req.Mode.Valid() {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid mode: " + string(req.Mode)})
			return
		}

		// Require flushing support — streaming is the whole point of this endpoint.
		// On unix sockets with chi, this is always satisfied; treat absence as a hard error.
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
				log.Printf("[daemon] ask: write event error: %v", err)
				cancel() // stop the agent loop — client is gone
				return
			}
			flusher.Flush()
		}

		if err := ask.RunAsk(ctx, req, cfg, emit); err != nil {
			// RunAsk already emits error events for agent errors.
			// This catches infrastructure errors (temenos unreachable, etc.)
			emit(ask.Event{Type: ask.EventError, Message: err.Error()})
		}
	}
}
