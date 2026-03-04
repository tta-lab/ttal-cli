package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Adapter communicates with Codex via its WebSocket JSON-RPC API server.
type Adapter struct {
	cfg            runtime.AdapterConfig
	client         *Client
	proc           *process
	events         chan runtime.Event
	conversationID string
	mu             sync.Mutex
	wg             sync.WaitGroup
	cancel         context.CancelFunc
}

// New creates a Codex adapter.
func New(cfg runtime.AdapterConfig) *Adapter {
	return &Adapter{
		cfg:    cfg,
		events: make(chan runtime.Event, 64),
	}
}

func (a *Adapter) Runtime() runtime.Runtime { return runtime.Codex }

func (a *Adapter) Start(ctx context.Context) error {
	a.proc = &process{
		port:    a.cfg.Port,
		workDir: a.cfg.WorkDir,
		env:     a.cfg.Env,
	}
	if err := a.proc.start(ctx); err != nil {
		return err
	}

	url := fmt.Sprintf("ws://127.0.0.1:%d", a.cfg.Port)
	client, err := NewClient(url)
	if err != nil {
		a.proc.stop()
		return fmt.Errorf("connect codex: %w", err)
	}
	a.client = client

	initParams := map[string]interface{}{
		"clientInfo": map[string]string{
			"name":    "ttal",
			"title":   "TTAL CLI",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"experimentalApi": true,
		},
	}
	if _, err := a.client.Call("initialize", initParams); err != nil {
		_ = a.client.Close()
		a.proc.stop()
		return fmt.Errorf("codex initialize: %w", err)
	}

	var notifCtx context.Context
	notifCtx, a.cancel = context.WithCancel(ctx)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.handleNotifications(notifCtx)
	}()

	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.client != nil {
		_ = a.client.Close()
	}
	a.wg.Wait()
	a.proc.stop()
	close(a.events)
	return nil
}

func (a *Adapter) SendMessage(_ context.Context, text string) error {
	a.mu.Lock()
	cid := a.conversationID
	a.mu.Unlock()

	if cid == "" {
		return fmt.Errorf("no active thread — call CreateSession first")
	}

	_, err := a.client.Call("turn/start", map[string]interface{}{
		"threadId": cid,
		"input": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	})
	return err
}

func (a *Adapter) Events() <-chan runtime.Event { return a.events }

func (a *Adapter) CreateSession(_ context.Context) (string, error) {
	params := map[string]interface{}{
		"cwd": a.cfg.WorkDir,
	}
	if a.cfg.Yolo {
		params["approvalPolicy"] = "never"
	}
	result, err := a.client.Call("thread/start", params)
	if err != nil {
		return "", fmt.Errorf("start codex thread: %w", err)
	}

	var resp struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	a.mu.Lock()
	a.conversationID = resp.Thread.ID
	a.mu.Unlock()

	return resp.Thread.ID, nil
}

func (a *Adapter) ResumeSession(_ context.Context, sessionID string) error {
	_, err := a.client.Call("thread/resume", map[string]interface{}{
		"threadId": sessionID,
	})
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.conversationID = sessionID
	a.mu.Unlock()
	return nil
}

// ListThreads returns the most recent thread ID for this agent's workdir, if any exist.
func (a *Adapter) ListThreads(_ context.Context) (string, error) {
	result, err := a.client.Call("thread/list", map[string]interface{}{
		"sortKey": "updated_at",
		"limit":   1,
		"cwd":     a.cfg.WorkDir,
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", nil
	}
	return resp.Data[0].ID, nil
}

func (a *Adapter) IsHealthy(_ context.Context) bool {
	if a.client == nil || a.proc == nil || !a.proc.isRunning() {
		return false
	}
	a.mu.Lock()
	cid := a.conversationID
	a.mu.Unlock()

	if cid == "" {
		return true // server up, no conversation yet
	}
	_, err := a.client.Call("thread/read", map[string]interface{}{
		"threadId": cid,
	})
	return err == nil
}

// RespondToUserInput sends a JSON-RPC response to the original server request.
func (a *Adapter) RespondToUserInput(callID string, answers []runtime.QuestionAnswer) error {
	id, err := strconv.ParseInt(callID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid request ID %q: %w", callID, err)
	}

	answerMap := make(map[string]interface{})
	for _, ans := range answers {
		answerMap[ans.QuestionID] = map[string]interface{}{
			"answers": []string{ans.Answer},
		}
	}

	return a.client.Respond(id, map[string]interface{}{
		"answers": answerMap,
	})
}

func (a *Adapter) handleNotifications(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case notif, ok := <-a.client.Notifications():
			if !ok {
				return
			}
			a.processNotification(notif)
		case req, ok := <-a.client.ServerRequests():
			if !ok {
				return
			}
			a.processServerRequest(req)
		}
	}
}

func (a *Adapter) sendEvent(evt runtime.Event) {
	select {
	case a.events <- evt:
	default:
		// drop if buffer full to avoid blocking the notification handler
	}
}

func (a *Adapter) processNotification(notif rpcResponse) {
	switch notif.Method {
	case "item/agentMessage/delta":
		// Streamed tokens — ignored in favor of item/completed for CC-like per-item delivery.

	case "item/completed":
		var params struct {
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal(notif.Params, &params) == nil && params.Item.Type == "agentMessage" && params.Item.Text != "" {
			a.sendEvent(runtime.Event{
				Type:  runtime.EventText,
				Agent: a.cfg.AgentName,
				Text:  params.Item.Text,
			})
		}

	case "turn/completed":
		a.sendEvent(runtime.Event{
			Type:  runtime.EventIdle,
			Agent: a.cfg.AgentName,
		})

	case "thread/status/changed":
		var params struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(notif.Params, &params) == nil && params.Status == "error" {
			a.sendEvent(runtime.Event{
				Type:  runtime.EventError,
				Agent: a.cfg.AgentName,
				Text:  "codex thread error",
			})
		}

	case "item/started":
		var params struct {
			Item struct {
				Type string `json:"type"`
			} `json:"item"`
		}
		if json.Unmarshal(notif.Params, &params) == nil && params.Item.Type != "" {
			a.sendEvent(runtime.Event{
				Type:     runtime.EventTool,
				Agent:    a.cfg.AgentName,
				ToolName: codexItemToToolName(params.Item.Type),
			})
		}

	default:
		log.Printf("[codex] unhandled notification: %s", notif.Method)
	}
}

func (a *Adapter) processServerRequest(req rpcResponse) {
	switch req.Method {
	case "item/tool/requestUserInput":
		var params struct {
			ThreadID  string `json:"threadId"`
			TurnID    string `json:"turnId"`
			ItemID    string `json:"itemId"`
			Questions []struct {
				ID       string `json:"id"`
				Header   string `json:"header"`
				Question string `json:"question"`
				IsOther  bool   `json:"isOther"`
				IsSecret bool   `json:"isSecret"`
				Options  []struct {
					Label       string `json:"label"`
					Description string `json:"description"`
				} `json:"options"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			log.Printf("[codex] failed to parse requestUserInput for %s: %v", a.cfg.AgentName, err)
			return
		}

		questions := make([]runtime.Question, 0, len(params.Questions))
		for _, q := range params.Questions {
			rq := runtime.Question{
				ID:          q.ID,
				Header:      q.Header,
				Text:        q.Question,
				AllowCustom: q.IsOther,
				IsSecret:    q.IsSecret,
			}
			for _, opt := range q.Options {
				rq.Options = append(rq.Options, runtime.QuestionOption{
					Label:       opt.Label,
					Description: opt.Description,
				})
			}
			questions = append(questions, rq)
		}

		if len(questions) > 0 {
			a.sendEvent(runtime.Event{
				Type:          runtime.EventQuestion,
				Agent:         a.cfg.AgentName,
				CorrelationID: fmt.Sprintf("%d", req.ID),
				Questions:     questions,
			})
		}

	default:
		log.Printf("[codex] unhandled server request: %s", req.Method)
	}
}

// codexItemToToolName maps Codex ThreadItem types to CC-compatible tool names
// so telegram.ToolEmoji() can handle both runtimes uniformly.
func codexItemToToolName(itemType string) string {
	switch itemType {
	case "commandExecution":
		return "Bash"
	case "fileChange":
		return "Edit"
	case "webSearch":
		return "WebSearch"
	case "mcpToolCall":
		return "MCP"
	default:
		return itemType
	}
}
