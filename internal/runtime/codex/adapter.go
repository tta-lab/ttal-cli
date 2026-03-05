package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/tta-lab/codex-server-go/protocol"
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

	title := "TTAL CLI"
	expAPI := true
	initParams := protocol.InitializeParams{
		ClientInfo: protocol.ClientInfo{
			Name:    "ttal",
			Title:   &title,
			Version: "1.0.0",
		},
		Capabilities: &protocol.InitializeCapabilities{
			ExperimentalAPI: &expAPI,
		},
	}
	if _, err := a.client.Call(protocol.MethodInitialize, initParams); err != nil {
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

	textInput := protocol.TextUserInput{Text: text, Type: "text"}
	inputData, err := json.Marshal(textInput)
	if err != nil {
		return fmt.Errorf("marshal text input: %w", err)
	}

	params := protocol.TurnStartParams{
		ThreadID: cid,
		Input:    []protocol.UserInput{{Type: "text", Data: inputData}},
	}
	_, err = a.client.Call(protocol.MethodTurnStart, params)
	return err
}

func (a *Adapter) Events() <-chan runtime.Event { return a.events }

func (a *Adapter) CreateSession(_ context.Context) (string, error) {
	params := protocol.ThreadStartParams{
		Cwd: &a.cfg.WorkDir,
	}
	result, err := a.client.Call(protocol.MethodThreadStart, params)
	if err != nil {
		return "", fmt.Errorf("start codex thread: %w", err)
	}

	var resp protocol.ThreadStartResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	a.mu.Lock()
	a.conversationID = resp.Thread.ID
	a.mu.Unlock()

	return resp.Thread.ID, nil
}

func (a *Adapter) ResumeSession(_ context.Context, sessionID string) (string, error) {
	result, err := a.client.Call(protocol.MethodThreadResume, protocol.ThreadResumeParams{
		ThreadID: sessionID,
	})
	if err != nil {
		return "", err
	}

	var resp protocol.ThreadResumeResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	a.mu.Lock()
	a.conversationID = resp.Thread.ID
	a.mu.Unlock()
	return resp.Thread.ID, nil
}

// ListThreads returns the most recent thread ID for this agent's workdir, if any exist.
func (a *Adapter) ListThreads(_ context.Context) (string, error) {
	sortKey := protocol.ThreadSortKeyUpdatedAt
	limit := int64(1)
	result, err := a.client.Call(protocol.MethodThreadList, protocol.ThreadListParams{
		SortKey: &sortKey,
		Limit:   &limit,
		Cwd:     &a.cfg.WorkDir,
	})
	if err != nil {
		return "", err
	}

	var resp protocol.ThreadListResponse
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
	_, err := a.client.Call(protocol.MethodThreadRead, protocol.ThreadReadParams{
		ThreadID: cid,
	})
	return err == nil
}

// RespondToUserInput sends a JSON-RPC response to the original server request.
func (a *Adapter) RespondToUserInput(callID string, answers []runtime.QuestionAnswer) error {
	answerMap := make(map[string]protocol.ToolRequestUserInputAnswer, len(answers))
	for _, ans := range answers {
		answerMap[ans.QuestionID] = protocol.ToolRequestUserInputAnswer{
			Answers: []string{ans.Answer},
		}
	}

	return a.client.Respond(json.RawMessage(callID), protocol.ToolRequestUserInputResponse{
		Answers: answerMap,
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
	case protocol.NotifItemAgentMessageDelta:
		// Streamed tokens — ignored in favor of item/completed for CC-like per-item delivery.

	case protocol.NotifItemCompleted:
		var params protocol.ItemCompletedNotification
		if json.Unmarshal(notif.Params, &params) == nil && params.Item.Type == "agentMessage" {
			if msg, err := params.Item.AsAgentMessage(); err == nil && msg.Text != "" {
				a.sendEvent(runtime.Event{
					Type:  runtime.EventText,
					Agent: a.cfg.AgentName,
					Text:  msg.Text,
				})
			}
		}

	case protocol.NotifTurnCompleted:
		a.sendEvent(runtime.Event{
			Type:  runtime.EventIdle,
			Agent: a.cfg.AgentName,
		})

	case protocol.NotifThreadStatusChanged:
		var params protocol.ThreadStatusChangedNotification
		if json.Unmarshal(notif.Params, &params) == nil && params.Status.Type == "error" {
			a.sendEvent(runtime.Event{
				Type:  runtime.EventError,
				Agent: a.cfg.AgentName,
				Text:  "codex thread error",
			})
		}

	case protocol.NotifTurnStarted:
		// Server acknowledgement — no action needed

	case protocol.NotifItemStarted:
		var params protocol.ItemStartedNotification
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
	case protocol.ReqItemToolRequestUserInput:
		var params protocol.ToolRequestUserInputParams
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
				AllowCustom: q.IsOther != nil && *q.IsOther,
				IsSecret:    q.IsSecret != nil && *q.IsSecret,
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
				CorrelationID: string(req.ID),
				Questions:     questions,
			})
		}

	case protocol.ReqItemCommandExecutionRequestApproval:
		// Auto-approve as fallback; config.toml handles policy globally
		// but resumed sessions may not reflect current config.
		if err := a.client.Respond(req.ID, protocol.CommandExecutionRequestApprovalResponse{
			Decision: json.RawMessage(`"acceptForSession"`),
		}); err != nil {
			log.Printf("[codex] failed to auto-approve %s for %s: %v", req.Method, a.cfg.AgentName, err)
		} else {
			log.Printf("[codex] auto-approved %s for %s", req.Method, a.cfg.AgentName)
		}

	case protocol.ReqItemFileChangeRequestApproval:
		if err := a.client.Respond(req.ID, protocol.FileChangeRequestApprovalResponse{
			Decision: json.RawMessage(`"acceptForSession"`),
		}); err != nil {
			log.Printf("[codex] failed to auto-approve %s for %s: %v", req.Method, a.cfg.AgentName, err)
		} else {
			log.Printf("[codex] auto-approved %s for %s", req.Method, a.cfg.AgentName)
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
