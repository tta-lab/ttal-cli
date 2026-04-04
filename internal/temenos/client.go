package temenos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const defaultSocketPath = "~/.temenos/daemon.sock"

// Client is a thin HTTP client for the temenos admin API over a unix socket.
type Client struct {
	socketPath string
	httpClient *http.Client
}

// New creates a Client for the temenos admin unix socket.
// If socketPath is empty, defaults to ~/.temenos/daemon.sock.
func New(socketPath string) *Client {
	if socketPath == "" {
		socketPath = defaultSocketPath
	}
	if len(socketPath) >= 2 && socketPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			socketPath = filepath.Join(home, socketPath[2:])
		}
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// registerRequest is the JSON body for POST /session/register.
type registerRequest struct {
	Agent      string   `json:"agent"`
	Access     string   `json:"access"`
	WritePaths []string `json:"write_paths"`
	ReadPaths  []string `json:"read_paths,omitempty"`
}

// registerResponse is the JSON body returned by POST /session/register.
type registerResponse struct {
	Token string `json:"token"`
}

// RegisterSession registers a new temenos session and returns the session token.
// agent is the agent identity name. writePaths are paths the session may write to.
// readPaths are additional read-only paths beyond the temenos baseline config.
func (c *Client) RegisterSession(ctx context.Context, agent string, writePaths, readPaths []string) (string, error) {
	body := registerRequest{
		Agent:      agent,
		Access:     "rw",
		WritePaths: writePaths,
		ReadPaths:  readPaths,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("temenos: marshal register request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://temenos/session/register", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("temenos: build register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("temenos: register session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("temenos: register session: unexpected status %d", resp.StatusCode)
	}

	var result registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("temenos: decode register response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("temenos: register session: empty token in response")
	}
	return result.Token, nil
}

// DeleteSession deletes a temenos session by token.
// Non-fatal in most contexts — callers should log on error and continue.
func (c *Client) DeleteSession(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://temenos/session/"+token, nil)
	if err != nil {
		return fmt.Errorf("temenos: build delete request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("temenos: delete session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("temenos: delete session: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Health checks that the temenos daemon is reachable.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://temenos/health", nil)
	if err != nil {
		return fmt.Errorf("temenos: build health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("temenos: health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("temenos: health check: unexpected status %d", resp.StatusCode)
	}
	return nil
}
