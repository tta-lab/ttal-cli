package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
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
	defer conn.Close() //nolint:errcheck

	conn.SetDeadline(time.Now().Add(socketTimeout)) //nolint:errcheck

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

// listenSocket starts the unix socket server and dispatches incoming requests
// to the handler function. Returns a cleanup function and any startup error.
func listenSocket(sockPath string, handler func(SendRequest) error) (func(), error) {
	// Remove stale socket file
	os.Remove(sockPath) //nolint:errcheck

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
			go handleConn(conn, handler)
		}
	}()

	cleanup := func() {
		ln.Close()          //nolint:errcheck
		os.Remove(sockPath) //nolint:errcheck
	}

	return cleanup, nil
}

func handleConn(conn net.Conn, handler func(SendRequest) error) {
	defer conn.Close()                              //nolint:errcheck
	conn.SetDeadline(time.Now().Add(socketTimeout)) //nolint:errcheck

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		errMsg := "empty request"
		if err := scanner.Err(); err != nil {
			errMsg = "read error: " + err.Error()
		}
		writeResponse(conn, SendResponse{OK: false, Error: errMsg})
		return
	}

	var req SendRequest
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		writeResponse(conn, SendResponse{OK: false, Error: "invalid JSON: " + err.Error()})
		return
	}

	if err := handler(req); err != nil {
		writeResponse(conn, SendResponse{OK: false, Error: err.Error()})
		return
	}

	writeResponse(conn, SendResponse{OK: true})
}

func writeResponse(conn net.Conn, resp SendResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	conn.Write(append(data, '\n')) //nolint:errcheck
}
