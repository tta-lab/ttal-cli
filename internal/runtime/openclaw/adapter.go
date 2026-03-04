package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Adapter delivers messages to OpenClaw agents via Gateway HTTP hooks.
// Send-only: POST /hooks/agent. OpenClaw owns human-facing messaging.
type Adapter struct {
	cfg        runtime.AdapterConfig
	sessionKey string // "agent:<name>:main"
	hooksURL   string // "<gateway>/hooks/agent"
	events     chan runtime.Event
	stopOnce   sync.Once
}

// New creates an OpenClaw adapter for the given agent.
func New(cfg runtime.AdapterConfig) *Adapter {
	return &Adapter{
		cfg:        cfg,
		sessionKey: "agent:" + cfg.AgentName + ":main",
		hooksURL:   cfg.GatewayURL + "/hooks/agent",
		events:     make(chan runtime.Event, 1),
	}
}

func (a *Adapter) Runtime() runtime.Runtime { return runtime.OpenClaw }

func (a *Adapter) Start(_ context.Context) error {
	// No connection to establish — HTTP is stateless.
	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	a.stopOnce.Do(func() { close(a.events) })
	return nil
}

func (a *Adapter) SendMessage(ctx context.Context, text string) error {
	body, err := json.Marshal(map[string]interface{}{
		"message":    text,
		"name":       "ttal",
		"sessionKey": a.sessionKey,
		"wakeMode":   "now",
		"deliver":    true,
	})
	if err != nil {
		return fmt.Errorf("marshal hook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.hooksURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.cfg.HooksToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.HooksToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", a.hooksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("hooks/agent returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Stubs — OpenClaw owns sessions and human messaging.

func (a *Adapter) Events() <-chan runtime.Event { return a.events }

func (a *Adapter) CreateSession(_ context.Context) (string, error) { return a.sessionKey, nil }

func (a *Adapter) ResumeSession(_ context.Context, _ string) (string, error) { return "", nil }

func (a *Adapter) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", a.cfg.GatewayURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
