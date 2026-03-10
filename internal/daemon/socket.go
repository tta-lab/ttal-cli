package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/status"
)

const socketTimeout = 5 * time.Second

// SocketPath returns the path to the daemon unix socket.
// Fixed at ~/.ttal/daemon.sock — one daemon serves all teams.
func SocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", "daemon.sock"), nil
}

// Request is the top-level socket message envelope.
// GetStatusRequest queries agent status from the daemon.
// Wire format: {"type":"getStatus","agent":"kestrel"}
type GetStatusRequest struct {
	Type  string `json:"type"`            // "getStatus"
	Team  string `json:"team,omitempty"`  // team name (defaults to "default")
	Agent string `json:"agent,omitempty"` // empty = all agents
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

// Send connects to the daemon socket and sends a message.
// Returns an error if the daemon is not running or if delivery fails.
// Auto-populates Team from TTAL_TEAM env if not set.
func Send(req SendRequest) error {
	if req.Team == "" {
		req.Team = os.Getenv("TTAL_TEAM")
	}

	sockPath, err := SocketPath()
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("unix", sockPath, socketTimeout)
	if err != nil {
		return fmt.Errorf("daemon not running (could not connect to %s): %w", sockPath, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(socketTimeout))

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading daemon response: %w", err)
		}
		return fmt.Errorf("no response from daemon")
	}

	var resp SendResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return fmt.Errorf("invalid response from daemon: %w", err)
	}

	if !resp.OK {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}

	return nil
}

// QueryStatus connects to the daemon socket and queries agent status.
func QueryStatus(team, agent string) (*StatusResponse, error) {
	sockPath, err := SocketPath()
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("unix", sockPath, socketTimeout)
	if err != nil {
		return nil, fmt.Errorf("daemon not running (could not connect to %s): %w", sockPath, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(socketTimeout))

	req := GetStatusRequest{Type: "getStatus", Team: team, Agent: agent}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading daemon response: %w", err)
		}
		return nil, fmt.Errorf("no response from daemon")
	}

	var resp StatusResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("invalid response from daemon: %w", err)
	}

	return &resp, nil
}

// socketHandlers groups all handler functions for the socket dispatcher.
type socketHandlers struct {
	send         func(SendRequest) error
	statusUpdate func(StatusUpdateRequest)
}

// listenSocket starts the unix socket server and dispatches incoming requests
// to the handler functions. Returns a cleanup function and any startup error.
func listenSocket(sockPath string, handlers socketHandlers) (func(), error) {
	// Remove stale socket file
	os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", sockPath, err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Listener closed — exit goroutine
				return
			}
			go handleConn(conn, handlers)
		}
	}()

	cleanup := func() {
		ln.Close()
	}

	return cleanup, nil
}

func handleConn(conn net.Conn, handlers socketHandlers) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(socketTimeout))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		errMsg := "empty request"
		if err := scanner.Err(); err != nil {
			errMsg = "read error: " + err.Error()
		}
		writeJSON(conn, SendResponse{OK: false, Error: errMsg})
		return
	}

	raw := scanner.Bytes()

	// Peek at type field to route the message.
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		writeJSON(conn, SendResponse{OK: false, Error: "invalid JSON: " + err.Error()})
		return
	}

	switch peek.Type {
	case "getStatus":
		writeJSON(conn, handleConnGetStatus(raw))
	case "statusUpdate":
		writeJSON(conn, handleConnStatusUpdate(raw, handlers))
	default:
		writeJSON(conn, handleConnSend(raw, handlers))
	}
}

func handleConnGetStatus(raw []byte) StatusResponse {
	var req GetStatusRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return StatusResponse{OK: false, Error: "invalid JSON: " + err.Error()}
	}
	team := req.Team
	if team == "" {
		team = config.DefaultTeamName
	}
	if req.Agent != "" {
		s, err := status.ReadAgent(team, req.Agent)
		if err != nil {
			return StatusResponse{OK: false, Error: err.Error()}
		}
		if s == nil {
			return StatusResponse{OK: true, Agents: nil}
		}
		return StatusResponse{OK: true, Agents: []status.AgentStatus{*s}}
	}

	all, err := status.ReadAll(team)
	if err != nil {
		return StatusResponse{OK: false, Error: err.Error()}
	}
	return StatusResponse{OK: true, Agents: all}
}

func handleConnStatusUpdate(raw []byte, handlers socketHandlers) SendResponse {
	var req StatusUpdateRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return SendResponse{OK: false, Error: "invalid statusUpdate JSON: " + err.Error()}
	}
	if handlers.statusUpdate != nil {
		handlers.statusUpdate(req)
	}
	return SendResponse{OK: true}
}

func handleConnSend(raw []byte, handlers socketHandlers) SendResponse {
	var req SendRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return SendResponse{OK: false, Error: "invalid JSON: " + err.Error()}
	}

	if err := handlers.send(req); err != nil {
		return SendResponse{OK: false, Error: err.Error()}
	}
	return SendResponse{OK: true}
}

func writeJSON(conn net.Conn, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	conn.Write(append(data, '\n'))
}
