package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	Team     string `json:"team,omitempty"`     // defaults to "default"
	Desc     string `json:"desc,omitempty"`     // task description for the notification message
	PRID     string `json:"pr_id,omitempty"`    // PR number for the notification message
	PRTitle  string `json:"pr_title,omitempty"` // PR title (preferred over Desc for notifications)
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

// PRCreateRequest asks the daemon to create a PR via the authenticated provider.
type PRCreateRequest struct {
	ProviderType string `json:"provider_type"` // "forgejo" or "github"
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Head         string `json:"head"` // source branch
	Base         string `json:"base"` // target branch
	Title        string `json:"title"`
	Body         string `json:"body"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRModifyRequest asks the daemon to edit a PR title/body.
type PRModifyRequest struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Index        int64  `json:"index"`
	Title        string `json:"title,omitempty"`
	Body         string `json:"body,omitempty"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRMergeRequest asks the daemon to squash-merge a PR.
type PRMergeRequest struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Index        int64  `json:"index"`
	DeleteBranch bool   `json:"delete_branch"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRCheckMergeableRequest asks the daemon to check if a PR is mergeable.
type PRCheckMergeableRequest struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Index        int64  `json:"index"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRGetPRRequest asks the daemon to fetch a PR (for HeadSHA resolution in CI commands).
type PRGetPRRequest struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Index        int64  `json:"index"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRGetCombinedStatusRequest asks the daemon to fetch CI status for a commit.
type PRGetCombinedStatusRequest struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	SHA          string `json:"sha"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRGetCIFailureDetailsRequest asks the daemon to fetch CI failure details.
type PRGetCIFailureDetailsRequest struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	SHA          string `json:"sha"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// PRResponse is the daemon's response for PR operations.
type PRResponse struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	PRURL         string `json:"pr_url,omitempty"`
	PRIndex       int64  `json:"pr_index,omitempty"`
	HeadSHA       string `json:"head_sha,omitempty"`
	AlreadyMerged bool   `json:"already_merged,omitempty"`
}

// PRGetPRResponse is the daemon's response for GetPR.
type PRGetPRResponse struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	HeadSHA   string `json:"head_sha,omitempty"`
	Merged    bool   `json:"merged,omitempty"`
	Mergeable bool   `json:"mergeable,omitempty"`
	Title     string `json:"title,omitempty"`
}

// PRCIStatusResponse is the daemon's response for GetCombinedStatus.
type PRCIStatusResponse struct {
	OK       bool         `json:"ok"`
	Error    string       `json:"error,omitempty"`
	State    string       `json:"state,omitempty"`
	Statuses []PRCIStatus `json:"statuses,omitempty"`
}

// PRCIStatus is a single CI check status.
type PRCIStatus struct {
	Context     string `json:"context"`
	State       string `json:"state"`
	Description string `json:"description"`
	TargetURL   string `json:"target_url"`
}

// PRCIFailureDetailsResponse is the daemon's response for GetCIFailureDetails.
type PRCIFailureDetailsResponse struct {
	OK      bool                `json:"ok"`
	Error   string              `json:"error,omitempty"`
	Details []PRCIFailureDetail `json:"details,omitempty"`
}

// PRCIFailureDetail is a single CI failure entry.
type PRCIFailureDetail struct {
	JobName      string `json:"job_name"`
	WorkflowName string `json:"workflow_name"`
	HTMLURL      string `json:"html_url"`
	LogTail      string `json:"log_tail"`
}

// BreatheRequest asks the daemon to restart an agent with a fresh context window.
type BreatheRequest struct {
	Team        string `json:"team,omitempty"`         // defaults to "default"
	Agent       string `json:"agent"`                  // agent name
	Handoff     string `json:"handoff"`                // handoff prompt content
	SessionName string `json:"session_name,omitempty"` // current tmux session name (if known)
}

// CommentAddRequest asks the daemon to add a comment to a task.
type CommentAddRequest struct {
	Target string `json:"target"` // taskwarrior task UUID
	Author string `json:"author"`
	Body   string `json:"body"`
	// Optional PR context for mirroring to remote PR
	ProviderType string `json:"provider_type,omitempty"`
	Owner        string `json:"owner,omitempty"`
	Repo         string `json:"repo,omitempty"`
	PRIndex      int64  `json:"pr_index,omitempty"`
	ProjectAlias string `json:"project_alias,omitempty"` // for per-project GitHub token resolution
}

// CommentAddResponse is the daemon's response for a comment add.
type CommentAddResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	ID    string `json:"id,omitempty"`
	Round int    `json:"round,omitempty"`
}

// CommentEntry is a single comment in a CommentListResponse.
type CommentEntry struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	Round     int    `json:"round"`
	CreatedAt string `json:"created_at"`
}

// CommentListRequest asks the daemon to list comments on a task.
type CommentListRequest struct {
	Target string `json:"target"` // taskwarrior task UUID
}

// CommentListResponse is the daemon's response for a comment list.
type CommentListResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error,omitempty"`
	Comments []CommentEntry `json:"comments,omitempty"`
}

// CommentGetRequest asks the daemon to get comments for a specific round.
// Team is omitted — daemon injects from mcfg.DefaultTeamName(), consistent with CommentListRequest.
type CommentGetRequest struct {
	Target string `json:"target"` // taskwarrior task UUID
	Round  int    `json:"round"`
}

// CommentGetResponse is the daemon's response for a comment get.
type CommentGetResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error,omitempty"`
	Comments []CommentEntry `json:"comments,omitempty"`
}

// CloseWindowRequest asks the daemon to close a tmux window.
// Used by ttal comment lgtm to auto-close the reviewer's window after LGTM.
type CloseWindowRequest struct {
	Session string `json:"session"` // tmux session name
	Window  string `json:"window"`  // tmux window name (reviewer agent name from pipelines.toml)
}

// httpHandlers groups all handler functions for the HTTP server.
// Unlike the old socketHandlers, taskComplete receives a typed struct
// instead of raw bytes — the HTTP layer handles JSON decoding.
type httpHandlers struct {
	send         func(SendRequest) error
	statusUpdate func(StatusUpdateRequest)
	taskComplete func(TaskCompleteRequest) SendResponse
	breathe      func(BreatheRequest) SendResponse
	askHuman     http.HandlerFunc
	// Pipeline advance (may block on human gates)
	pipelineAdvance http.HandlerFunc
	// Comment operations (stored in ttal DB)
	commentAdd  func(CommentAddRequest) CommentAddResponse
	commentList func(CommentListRequest) CommentListResponse
	commentGet  func(CommentGetRequest) CommentGetResponse
	// Window lifecycle
	closeWindow func(CloseWindowRequest) SendResponse
	// PR operations (daemon-proxied for token isolation)
	prCreate              func(PRCreateRequest) PRResponse
	prModify              func(PRModifyRequest) PRResponse
	prMerge               func(PRMergeRequest) PRResponse
	prCheckMergeable      func(PRCheckMergeableRequest) PRResponse
	prGetPR               func(PRGetPRRequest) PRGetPRResponse
	prGetCombinedStatus   func(PRGetCombinedStatusRequest) PRCIStatusResponse
	prGetCIFailureDetails func(PRGetCIFailureDetailsRequest) PRCIFailureDetailsResponse
}

// newDaemonRouter creates the chi router with all daemon routes.
func newDaemonRouter(handlers httpHandlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Post("/send", handleHTTPSend(handlers))
	r.Get("/status", handleHTTPGetStatus())
	r.Post("/status/update", handleHTTPStatusUpdate(handlers))
	r.Post("/task/complete", handleHTTPTaskComplete(handlers))
	r.Post("/breathe", handleHTTPBreathe(handlers))
	r.Post("/ask/human", handlers.askHuman)
	r.Post("/pipeline/advance", handlers.pipelineAdvance)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeHTTPJSON(w, http.StatusOK, SendResponse{OK: true})
	})
	// Comment routes (stored in ttal DB)
	r.Post("/comment/add", handleHTTPCommentAdd(handlers))
	r.Post("/comment/list", handleHTTPCommentList(handlers))
	r.Post("/comment/get", handleHTTPCommentGet(handlers))
	// Window lifecycle
	r.Post("/window/close", handleHTTPCloseWindow(handlers))
	// PR routes (proxied through daemon for token isolation)
	r.Post("/pr/create", handleHTTPPR("prCreate", handlers.prCreate))
	r.Post("/pr/modify", handleHTTPPR("prModify", handlers.prModify))
	r.Post("/pr/merge", handleHTTPPR("prMerge", handlers.prMerge))
	r.Post("/pr/check-mergeable", handleHTTPPR("prCheckMergeable", handlers.prCheckMergeable))
	r.Post("/pr/get", handleHTTPPRGetPR(handlers))
	r.Post("/pr/ci/status", handleHTTPPRCIStatus(handlers))
	r.Post("/pr/ci/failure-details", handleHTTPPRCIFailureDetails(handlers))
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

// handleHTTPWithResponse creates a typed HTTP handler: decode JSON, call fn, map OK/error to status code.
// fn must be non-nil; callers are responsible for always populating httpHandlers fields.
func handleHTTPWithResponse[Req any](name string, fn func(Req) SendResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				SendResponse{OK: false, Error: "invalid " + name + " JSON: " + err.Error()})
			return
		}
		result := fn(req)
		code := http.StatusOK
		if !result.OK {
			code = http.StatusInternalServerError
		}
		writeHTTPJSON(w, code, result)
	}
}

func handleHTTPTaskComplete(handlers httpHandlers) http.HandlerFunc {
	return handleHTTPWithResponse("taskComplete", handlers.taskComplete)
}

func handleHTTPBreathe(handlers httpHandlers) http.HandlerFunc {
	return handleHTTPWithResponse("breathe", handlers.breathe)
}

func handleHTTPCommentAdd(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CommentAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				CommentAddResponse{OK: false, Error: "invalid commentAdd JSON: " + err.Error()})
			return
		}
		result := handlers.commentAdd(req)
		code := http.StatusOK
		if !result.OK {
			code = http.StatusInternalServerError
		}
		writeHTTPJSON(w, code, result)
	}
}

func handleHTTPCommentList(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CommentListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				CommentListResponse{OK: false, Error: "invalid commentList JSON: " + err.Error()})
			return
		}
		result := handlers.commentList(req)
		code := http.StatusOK
		if !result.OK {
			code = http.StatusInternalServerError
		}
		writeHTTPJSON(w, code, result)
	}
}

func handleHTTPCommentGet(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CommentGetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				CommentGetResponse{OK: false, Error: "invalid commentGet JSON: " + err.Error()})
			return
		}
		result := handlers.commentGet(req)
		code := http.StatusOK
		if !result.OK {
			code = http.StatusInternalServerError
		}
		writeHTTPJSON(w, code, result)
	}
}

func handleHTTPCloseWindow(handlers httpHandlers) http.HandlerFunc {
	return handleHTTPWithResponse("closeWindow", handlers.closeWindow)
}

// prOKStatus maps an OK flag to an HTTP status code.
func prOKStatus(ok bool) int {
	if ok {
		return http.StatusOK
	}
	return http.StatusInternalServerError
}

// handleHTTPPR creates a typed HTTP handler for PR operations returning PRResponse.
func handleHTTPPR[Req any](name string, fn func(Req) PRResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				PRResponse{OK: false, Error: "invalid " + name + " JSON: " + err.Error()})
			return
		}
		result := fn(req)
		writeHTTPJSON(w, prOKStatus(result.OK), result)
	}
}

func handleHTTPPRGetPR(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PRGetPRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				PRGetPRResponse{OK: false, Error: "invalid prGetPR JSON: " + err.Error()})
			return
		}
		result := handlers.prGetPR(req)
		writeHTTPJSON(w, prOKStatus(result.OK), result)
	}
}

func handleHTTPPRCIStatus(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PRGetCombinedStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				PRCIStatusResponse{OK: false, Error: "invalid prGetCombinedStatus JSON: " + err.Error()})
			return
		}
		result := handlers.prGetCombinedStatus(req)
		writeHTTPJSON(w, prOKStatus(result.OK), result)
	}
}

func handleHTTPPRCIFailureDetails(handlers httpHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PRGetCIFailureDetailsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest,
				PRCIFailureDetailsResponse{OK: false, Error: "invalid prGetCIFailureDetails JSON: " + err.Error()})
			return
		}
		result := handlers.prGetCIFailureDetails(req)
		writeHTTPJSON(w, prOKStatus(result.OK), result)
	}
}

// prCall is the generic helper for PR operations returning PRResponse.
func prCall[Req any](path string, req Req) (PRResponse, error) {
	return prCallTyped(path, req, func(r PRResponse) string { return r.Error })
}

// prCallTyped is the generic helper for PR operations returning a typed response.
// getErr extracts the error string from the response type for error propagation.
// Uses a 30-second timeout (vs the default 5s) since PR operations involve network
// API calls. Makes up to 3 attempts (2 retries) with exponential backoff for transient
// connection errors (e.g. daemon restart), but not for timeout errors (daemon running but slow).
func prCallTyped[Req any, Resp any](path string, req Req, getErr func(Resp) string) (Resp, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return *new(Resp), fmt.Errorf("marshal PR request: %w", err)
	}

	client := daemonHTTPClientLong(prClientTimeout)

	// Retry with backoff for transient connection errors (e.g. daemon restart).
	// Only retry on dial/connection errors — NOT on timeouts, which indicate
	// the daemon is running but slow (retrying would triple the wait time).
	var resp *http.Response
	backoff := 1 * time.Second
	const maxRetries = 3
	for attempt := range maxRetries {
		resp, err = client.Post(daemonBaseURL+path, "application/json", bytes.NewReader(body))
		if err == nil {
			break
		}
		// Don't retry timeout errors — the daemon is running but slow.
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			break
		}
		// Last attempt — don't sleep, just exit with the error.
		if attempt == maxRetries-1 {
			break
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return *new(Resp), fmt.Errorf(
				"PR operation timed out after %s — daemon is running but slow: %w", prClientTimeout, err)
		}
		return *new(Resp), fmt.Errorf("daemon not running — ttal pr requires the daemon: %w", err)
	}
	defer resp.Body.Close()
	var result Resp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return *new(Resp), fmt.Errorf("decode PR response: %w", err)
	}
	if errMsg := getErr(result); errMsg != "" {
		return result, fmt.Errorf("%s", errMsg) //nolint:err113
	}
	return result, nil
}

// PRCreate asks the daemon to create a PR via the authenticated provider.
func PRCreate(req PRCreateRequest) (PRResponse, error) {
	return prCall("/pr/create", req)
}

// PRModify asks the daemon to edit a PR title/body.
func PRModify(req PRModifyRequest) (PRResponse, error) {
	return prCall("/pr/modify", req)
}

// PRMerge asks the daemon to squash-merge a PR.
func PRMerge(req PRMergeRequest) (PRResponse, error) {
	return prCall("/pr/merge", req)
}

// PRCheckMergeable asks the daemon to check if a PR is mergeable.
func PRCheckMergeable(req PRCheckMergeableRequest) (PRResponse, error) {
	return prCall("/pr/check-mergeable", req)
}

// PRGetPR asks the daemon to fetch a PR.
func PRGetPR(req PRGetPRRequest) (PRGetPRResponse, error) {
	return prCallTyped("/pr/get", req, func(r PRGetPRResponse) string { return r.Error })
}

// PRGetCombinedStatus asks the daemon to fetch CI status for a commit.
func PRGetCombinedStatus(req PRGetCombinedStatusRequest) (PRCIStatusResponse, error) {
	return prCallTyped("/pr/ci/status", req, func(r PRCIStatusResponse) string { return r.Error })
}

// PRGetCIFailureDetails asks the daemon to fetch CI failure details.
func PRGetCIFailureDetails(req PRGetCIFailureDetailsRequest) (PRCIFailureDetailsResponse, error) {
	return prCallTyped("/pr/ci/failure-details", req, func(r PRCIFailureDetailsResponse) string { return r.Error })
}

// CommentAdd asks the daemon to add a comment to a task.
func CommentAdd(req CommentAddRequest) (CommentAddResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return CommentAddResponse{}, fmt.Errorf("marshal comment add request: %w", err)
	}
	client := daemonHTTPClient()
	resp, err := client.Post(daemonBaseURL+"/comment/add", "application/json", bytes.NewReader(body))
	if err != nil {
		return CommentAddResponse{}, fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()
	var result CommentAddResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CommentAddResponse{}, fmt.Errorf("decode comment add response: %w", err)
	}
	if !result.OK {
		return result, fmt.Errorf("%s", result.Error) //nolint:err113
	}
	return result, nil
}

// CommentGet asks the daemon for comments at a specific round.
func CommentGet(req CommentGetRequest) (CommentGetResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return CommentGetResponse{}, fmt.Errorf("marshal comment get request: %w", err)
	}
	client := daemonHTTPClient()
	resp, err := client.Post(daemonBaseURL+"/comment/get", "application/json", bytes.NewReader(body))
	if err != nil {
		return CommentGetResponse{}, fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return CommentGetResponse{}, fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}
	var result CommentGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CommentGetResponse{}, fmt.Errorf("decode comment get response: %w", err)
	}
	if !result.OK {
		return result, fmt.Errorf("%s", result.Error) //nolint:err113
	}
	return result, nil
}

// CommentList asks the daemon to list comments on a task.
func CommentList(req CommentListRequest) (CommentListResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return CommentListResponse{}, fmt.Errorf("marshal comment list request: %w", err)
	}
	client := daemonHTTPClient()
	resp, err := client.Post(daemonBaseURL+"/comment/list", "application/json", bytes.NewReader(body))
	if err != nil {
		return CommentListResponse{}, fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()
	var result CommentListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CommentListResponse{}, fmt.Errorf("decode comment list response: %w", err)
	}
	if !result.OK {
		return result, fmt.Errorf("%s", result.Error) //nolint:err113
	}
	return result, nil
}

// CloseWindow asks the daemon to close a tmux window.
// Fire-and-forget: errors are returned but callers should treat them as non-fatal.
func CloseWindow(req CloseWindowRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal close window request: %w", err)
	}
	client := daemonHTTPClient()
	resp, err := client.Post(daemonBaseURL+"/window/close", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()
	var result SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode close window response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("close window: %s", result.Error) //nolint:err113
	}
	return nil
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

// daemonHTTPClientLong returns an http.Client with a custom timeout for long-blocking
// requests (e.g. /ask/human which waits up to 5 minutes for a human reply).
func daemonHTTPClientLong(timeout time.Duration) *http.Client {
	sockPath, _ := SocketPath()
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", sockPath, socketTimeout)
			},
		},
	}
}

// Send connects to the daemon socket and sends a message via HTTP.
// Returns an error if the daemon is not running or if delivery fails.
func Send(req SendRequest) error {
	if req.Team == "" {
		req.Team = config.DefaultTeamName
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

// prClientTimeout is the total request timeout for PR operations.
// PR creation involves a network API call to Forgejo/GitHub which can take
// several seconds, so we use a generous timeout to avoid spurious failures.
const prClientTimeout = 30 * time.Second

// askHumanClientTimeout is the total request timeout for /ask/human.
// 30 seconds longer than the daemon's 5-minute question timeout to allow the
// daemon to write its response before the client gives up.
const askHumanClientTimeout = 5*time.Minute + 30*time.Second

// AskHuman sends a question to a human via Telegram and blocks until answered.
// Returns the response with the human's answer, or Skipped=true on timeout/skip.
func AskHuman(req AskHumanRequest) (AskHumanResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return AskHumanResponse{}, fmt.Errorf("encode request: %w", err)
	}

	client := daemonHTTPClientLong(askHumanClientTimeout)
	resp, err := client.Post(daemonBaseURL+"/ask/human", "application/json", bytes.NewReader(body))
	if err != nil {
		return AskHumanResponse{}, fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()

	var result AskHumanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AskHumanResponse{}, fmt.Errorf("invalid response from daemon (HTTP %d): %w", resp.StatusCode, err)
	}
	if result.Error != "" {
		return AskHumanResponse{}, fmt.Errorf("daemon error: %s", result.Error)
	}
	if !result.OK && !result.Skipped {
		return AskHumanResponse{}, fmt.Errorf("daemon returned failure without details")
	}
	return result, nil
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

// Breathe sends a breathe request to the daemon, asking it to restart an agent's
// CC session with a fresh context window and the provided handoff prompt.
func Breathe(req BreatheRequest) error {
	if req.Team == "" {
		req.Team = config.DefaultTeamName
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	client := daemonHTTPClient()
	resp, err := client.Post(daemonBaseURL+"/breathe", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()
	var result SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("invalid response from daemon: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("breathe failed: %s", result.Error)
	}
	return nil
}
