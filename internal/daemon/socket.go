package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/status"
)

const socketTimeout = 5 * time.Second

// SocketPath returns the path to the daemon unix socket.
func SocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", "daemon.sock"), nil
}

// Request is the top-level socket message envelope.
// Type discriminates between request kinds. Empty type is treated as "send"
// for backwards compatibility with existing ttal send clients.
type Request struct {
	Type   string         `json:"type,omitempty"` // "send" (default) or "status"
	Send   *SendRequest   `json:"send,omitempty"`
	Status *StatusRequest `json:"status,omitempty"`
}

// SendRequest is the JSON message sent to the daemon.
// Direction is determined by which fields are set:
//
//	From only:       agent → human via Telegram
//	To only:         system/hook → agent via Zellij
//	From + To:       agent → agent via Zellij with attribution
type SendRequest struct {
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Message string `json:"message"`
}

// SendResponse is the JSON reply from the daemon.
type SendResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// StatusRequest asks for agent status data.
type StatusRequest struct {
	Agent string `json:"agent"` // empty = all agents
}

// StatusResponse returns agent status data.
type StatusResponse struct {
	OK     bool                 `json:"ok"`
	Agents []status.AgentStatus `json:"agents,omitempty"`
	Error  string               `json:"error,omitempty"`
}

// Send connects to the daemon socket and sends a message.
// Returns an error if the daemon is not running or if delivery fails.
func Send(req SendRequest) error {
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
func QueryStatus(agent string) (*StatusResponse, error) {
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

	envelope := Request{
		Type:   "status",
		Status: &StatusRequest{Agent: agent},
	}
	data, err := json.Marshal(envelope)
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

// listenSocket starts the unix socket server and dispatches incoming requests
// to the handler function. Returns a cleanup function and any startup error.
func listenSocket(sockPath string, sendHandler func(SendRequest) error) (func(), error) {
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
			go handleConn(conn, sendHandler)
		}
	}()

	cleanup := func() {
		ln.Close()
		os.Remove(sockPath)
	}

	return cleanup, nil
}

func handleConn(conn net.Conn, sendHandler func(SendRequest) error) {
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

	// Try to parse as Request envelope first
	var envelope Request
	if err := json.Unmarshal(raw, &envelope); err != nil {
		writeJSON(conn, SendResponse{OK: false, Error: "invalid JSON: " + err.Error()})
		return
	}

	switch envelope.Type {
	case "status":
		if envelope.Status == nil {
			writeJSON(conn, StatusResponse{OK: false, Error: "missing status field"})
			return
		}
		writeJSON(conn, handleStatusRequest(envelope.Status))

	default:
		// Backwards compat: no type or type="send" → treat as SendRequest.
		// If envelope.Send is populated, use it. Otherwise, re-parse as bare SendRequest.
		var req SendRequest
		if envelope.Send != nil {
			req = *envelope.Send
		} else if err := json.Unmarshal(raw, &req); err != nil {
			writeJSON(conn, SendResponse{OK: false, Error: "invalid JSON: " + err.Error()})
			return
		}

		if err := sendHandler(req); err != nil {
			writeJSON(conn, SendResponse{OK: false, Error: err.Error()})
			return
		}
		writeJSON(conn, SendResponse{OK: true})
	}
}

func handleStatusRequest(req *StatusRequest) StatusResponse {
	if req.Agent != "" {
		s, err := status.ReadAgent(req.Agent)
		if err != nil {
			return StatusResponse{OK: false, Error: err.Error()}
		}
		if s == nil {
			return StatusResponse{OK: true, Agents: nil}
		}
		return StatusResponse{OK: true, Agents: []status.AgentStatus{*s}}
	}

	all, err := status.ReadAll()
	if err != nil {
		return StatusResponse{OK: false, Error: err.Error()}
	}
	return StatusResponse{OK: true, Agents: all}
}

func writeJSON(conn net.Conn, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	conn.Write(append(data, '\n'))
}
