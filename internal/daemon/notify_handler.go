package daemon

import (
	"encoding/json"
	"net/http"
)

// handleHTTPNotify handles POST /notify.
// It accepts a NotifyRequest and routes the message through the frontend abstraction.
func handleHTTPNotify(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req NotifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest, SendResponse{Error: "bad json"})
			return
		}
		if err := handlers.notify(req.Team, req.Message); err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, SendResponse{Error: err.Error()})
			return
		}
		writeHTTPJSON(w, http.StatusOK, SendResponse{OK: true})
	}
}
