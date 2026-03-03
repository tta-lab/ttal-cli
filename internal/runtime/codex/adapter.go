package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	}
	if a.cfg.Yolo {
		initParams["approval_policy"] = "full-auto"
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
		return fmt.Errorf("no active conversation — call CreateSession first")
	}

	_, err := a.client.Call("sendUserTurn", map[string]interface{}{
		"conversation_id": cid,
		"content":         text,
	})
	return err
}

func (a *Adapter) Events() <-chan runtime.Event { return a.events }

func (a *Adapter) CreateSession(_ context.Context) (string, error) {
	result, err := a.client.Call("newConversation", map[string]interface{}{
		"working_directory": a.cfg.WorkDir,
	})
	if err != nil {
		return "", fmt.Errorf("create codex conversation: %w", err)
	}

	var conv struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(result, &conv); err != nil {
		return "", err
	}

	a.mu.Lock()
	a.conversationID = conv.ConversationID
	a.mu.Unlock()

	return conv.ConversationID, nil
}

func (a *Adapter) ResumeSession(_ context.Context, sessionID string) error {
	_, err := a.client.Call("resumeConversation", map[string]interface{}{
		"conversation_id": sessionID,
	})
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.conversationID = sessionID
	a.mu.Unlock()
	return nil
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
	_, err := a.client.Call("getConversationSummary", map[string]interface{}{
		"conversation_id": cid,
	})
	return err == nil
}

// RespondToUserInput sends answers back to Codex via JSON-RPC.
func (a *Adapter) RespondToUserInput(callID string, answers []runtime.QuestionAnswer) error {
	answerMap := make(map[string]interface{})
	for _, ans := range answers {
		answerMap[ans.QuestionID] = map[string]string{"answer": ans.Answer}
	}
	_, err := a.client.Call("UserInputResponse", map[string]interface{}{
		"id":       callID,
		"response": map[string]interface{}{"answers": answerMap},
	})
	return err
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
		var params struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(notif.Params, &params) == nil && params.Delta != "" {
			a.sendEvent(runtime.Event{
				Type:  runtime.EventText,
				Agent: a.cfg.AgentName,
				Text:  params.Delta,
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

	case "RequestUserInput":
		var params struct {
			CallID    string `json:"call_id"`
			Questions []struct {
				ID       string `json:"id"`
				Header   string `json:"header"`
				Question string `json:"question"`
				IsOther  bool   `json:"is_other"`
				IsSecret bool   `json:"is_secret"`
				Options  []struct {
					Label       string `json:"label"`
					Description string `json:"description"`
				} `json:"options"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			log.Printf("[codex] failed to parse RequestUserInput for %s: %v", a.cfg.AgentName, err)
			return
		}

		var questions []runtime.Question
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
				CorrelationID: params.CallID,
				Questions:     questions,
			})
		}

	case "item/started":
		var params struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(notif.Params, &params) == nil && params.Type != "" {
			a.sendEvent(runtime.Event{
				Type:     runtime.EventTool,
				Agent:    a.cfg.AgentName,
				ToolName: codexItemToToolName(params.Type),
			})
		}

	default:
		log.Printf("[codex] unhandled notification: %s", notif.Method)
	}
}

// codexItemToToolName maps Codex ThreadItem types to CC-compatible tool names
// so telegram.ToolEmoji() can handle both runtimes uniformly.
func codexItemToToolName(itemType string) string {
	switch itemType {
	case "command_execution":
		return "Bash"
	case "file_change":
		return "Edit"
	case "web_search":
		return "WebSearch"
	default:
		return itemType
	}
}
