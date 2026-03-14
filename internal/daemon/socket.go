package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/status"
)

const socketTimeout = 5 * time.Second

// SocketPath returns the path to the daemon unix socket.
// TTAL_SOCKET_PATH overrides the default.
// Delegates to config.SocketPath() to keep a single source of truth.
func SocketPath() (string, error) {
	return config.SocketPath(), nil
}

// StatusUpdateRequest writes agent context status to the daemon.
// Wire format: {"type":"statusUpdate","agent":"kestrel","context_used_pct":45.2,...}
type StatusUpdateRequest struct {
	Type                string  `json:"type"`                  // "statusUpdate"
	Team                string  `json:"team,omitempty"`        // team name (defaults to "default")
	Agent               string  `json:"agent"`                 // agent name
	ContextUsedPct      float64 `json:"context_used_pct"`      // percentage of context used
	ContextRemainingPct float64 `json:"context_remaining_pct"` // percentage remaining
	ModelID             string  `json:"model_id"`              // model identifier
	SessionID           string  `json:"session_id"`            // session identifier
}

// SendRequest is the JSON message sent to the daemon.
// Direction is determined by which fields are set:
//
//	From only:       agent → human via Telegram
//	To only:         system/hook → agent via tmux
//	From + To:       agent → agent via tmux with attribution
//
// Team disambiguates when agent names collide across teams.
// Auto-populated from TTAL_TEAM env if unset.
type SendRequest struct {
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Team    string `json:"team,omitempty"`
	Message string `json:"message"`
}

// TaskCompleteRequest notifies the daemon that a task has been marked done.
// Wire format: {"type":"taskComplete","task_uuid":"...","team":"default",...}
type TaskCompleteRequest struct {
	Type     string `json:"type"` // "taskComplete"
	TaskUUID string `json:"task_uuid"`
	Team     string `json:"team,omitempty"`    // defaults to "default"
	Spawner  string `json:"spawner,omitempty"` // "team:agent", optional
	Desc     string `json:"desc,omitempty"`    // task description for the notification message
	PRID     string `json:"pr_id,omitempty"`   // PR number for the notification message
}

// SendResponse is the JSON reply from the daemon.
type SendResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// StatusResponse returns agent status data.
type StatusResponse struct {
	OK     bool                 `json:"ok"`
	Agents []status.AgentStatus `json:"agents,omitempty"`
	Error  string               `json:"error,omitempty"`
}

// httpHandlers groups all handler functions for the HTTP server.
// Unlike the old socketHandlers, taskComplete receives a typed struct
// instead of raw bytes — the HTTP layer handles JSON decoding.
type httpHandlers struct {
	send         func(SendRequest) error
	statusUpdate func(StatusUpdateRequest)
	taskComplete func(TaskCompleteRequest) SendResponse
}

// newDaemonRouter creates the chi router with all daemon routes.
func newDaemonRouter(handlers httpHandlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Post("/send", handleHTTPSend(handlers))
	r.Get("/status", handleHTTPGetStatus())
	r.Post("/status/update", handleHTTPStatusUpdate(handlers))
	r.Post("/task/complete", handleHTTPTaskComplete(handlers))
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeHTTPJSON(w, http.StatusOK, SendResponse{OK: true})
	})
	return r
}

func handleHTTPSend(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid JSON: " + err.Error()})
			return
		}
		if err := handlers.send(req); err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError,
				SendResponse{OK: false, Error: err.Error()})
			return
		}
		writeHTTPJSON(w, http.StatusOK, SendResponse{OK: true})
	}
}

func handleHTTPGetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		team := r.URL.Query().Get("team")
		agent := r.URL.Query().Get("agent")
		if team == "" {
			team = config.DefaultTeamName
		}

		var resp StatusResponse
		if agent != "" {
			s, err := status.ReadAgent(team, agent)
			if err != nil {
				writeHTTPJSON(w, http.StatusInternalServerError,
					StatusResponse{OK: false, Error: err.Error()})
				return
			}
			if s == nil {
				resp = StatusResponse{OK: true, Agents: nil}
			} else {
				resp = StatusResponse{OK: true, Agents: []status.AgentStatus{*s}}
			}
		} else {
			all, err := status.ReadAll(team)
			if err != nil {
				writeHTTPJSON(w, http.StatusInternalServerError,
					StatusResponse{OK: false, Error: err.Error()})
				return
			}
			resp = StatusResponse{OK: true, Agents: all}
		}
		writeHTTPJSON(w, http.StatusOK, resp)
	}
}

func handleHTTPStatusUpdate(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req StatusUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid statusUpdate JSON: " + err.Error()})
			return
		}
		if handlers.statusUpdate != nil {
			handlers.statusUpdate(req)
		}
		writeHTTPJSON(w, http.StatusOK, SendResponse{OK: true})
	}
}

func handleHTTPTaskComplete(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req TaskCompleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid taskComplete JSON: " + err.Error()})
			return
		}
		if handlers.taskComplete != nil {
			resp := handlers.taskComplete(req)
			code := http.StatusOK
			if !resp.OK {
				code = http.StatusInternalServerError
			}
			writeHTTPJSON(w, code, resp)
		} else {
			writeHTTPJSON(w, http.StatusNotImplemented,
				SendResponse{OK: false, Error: "taskComplete handler not registered"})
		}
	}
}

func writeHTTPJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[daemon] writeHTTPJSON: failed to encode response: %v", err)
	}
}

// listenHTTP starts the chi HTTP server on a unix socket.
// Returns the server and any startup error.
func listenHTTP(sockPath string, handlers httpHandlers) (*http.Server, error) {
	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		log.Printf("[daemon] warning: could not remove stale socket %s: %v", sockPath, err)
	}

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", sockPath, err)
	}
	if err := os.Chmod(sockPath, 0o600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("insecure socket permissions: %w", err)
	}

	router := newDaemonRouter(handlers)
	srv := &http.Server{Handler: router}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[daemon] HTTP server error: %v", err)
		}
	}()

	return srv, nil
}

// daemonBaseURL is the HTTP base URL for the daemon server.
// The host is ignored — connections go via unix socket.
const daemonBaseURL = "http://daemon"

// daemonHTTPClient returns an http.Client configured to connect via unix socket.
// Note: SocketPath() wraps config.SocketPath() which always succeeds (returns
// a default path on error), so the error discard is safe.
func daemonHTTPClient() *http.Client {
	sockPath, _ := SocketPath()
	return &http.Client{
		Timeout: socketTimeout,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", sockPath, socketTimeout)
			},
		},
	}
}

// Send connects to the daemon socket and sends a message via HTTP.
// Returns an error if the daemon is not running or if delivery fails.
// Auto-populates Team from TTAL_TEAM env if not set.
func Send(req SendRequest) error {
	if req.Team == "" {
		req.Team = os.Getenv("TTAL_TEAM")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	client := daemonHTTPClient()
	resp, err := client.Post(daemonBaseURL+"/send", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()

	var result SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("invalid response from daemon: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("daemon error: %s", result.Error)
	}
	return nil
}

// QueryStatus connects to the daemon and queries agent status via HTTP.
func QueryStatus(team, agent string) (*StatusResponse, error) {
	client := daemonHTTPClient()

	params := url.Values{}
	if team != "" {
		params.Set("team", team)
	}
	if agent != "" {
		params.Set("agent", agent)
	}

	reqURL := daemonBaseURL + "/status"
	if encoded := params.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}

	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()

	var result StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response from daemon: %w", err)
	}
	return &result, nil
}
