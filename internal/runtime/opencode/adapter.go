package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	oc "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Adapter communicates with OpenCode via its HTTP API server.
type Adapter struct {
	cfg       runtime.AdapterConfig
	client    *oc.Client
	proc      *process
	events    chan runtime.Event
	sessionID string
	mu        sync.Mutex
	wg        sync.WaitGroup
	cancel    context.CancelFunc
}

// New creates an OpenCode adapter.
func New(cfg runtime.AdapterConfig) *Adapter {
	return &Adapter{
		cfg:    cfg,
		events: make(chan runtime.Event, 64),
	}
}

func (a *Adapter) Runtime() runtime.Runtime { return runtime.OpenCode }

func (a *Adapter) Start(ctx context.Context) error {
	env := append([]string{}, a.cfg.Env...)
	if a.cfg.Yolo {
		env = append(env, //nolint:lll // JSON env var
			`OPENCODE_PERMISSION={"bash":"allow","edit":"allow","read":"allow","write":"allow","question":"allow"}`)
	}

	a.proc = &process{
		port:    a.cfg.Port,
		workDir: a.cfg.WorkDir,
		env:     env,
	}
	if err := a.proc.start(ctx); err != nil {
		return err
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", a.cfg.Port)
	a.client = oc.NewClient(
		option.WithBaseURL(baseURL),
	)

	var streamCtx context.Context
	streamCtx, a.cancel = context.WithCancel(ctx)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.streamEvents(streamCtx)
	}()

	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	a.proc.stop()
	close(a.events)
	return nil
}

func (a *Adapter) SendMessage(ctx context.Context, text string) error {
	a.mu.Lock()
	sid := a.sessionID
	a.mu.Unlock()

	if sid == "" {
		return fmt.Errorf("no active session — call CreateSession first")
	}

	// NoReply = true for fire-and-forget; response comes via SSE.
	_, err := a.client.Session.Prompt(ctx, sid, oc.SessionPromptParams{
		Parts: oc.F([]oc.SessionPromptParamsPartUnion{
			oc.TextPartInputParam{
				Text: oc.F(text),
				Type: oc.F(oc.TextPartInputTypeText),
			},
		}),
		NoReply: oc.F(true),
	})
	return err
}

func (a *Adapter) Events() <-chan runtime.Event {
	return a.events
}

func (a *Adapter) CreateSession(ctx context.Context) (string, error) {
	session, err := a.client.Session.New(ctx, oc.SessionNewParams{})
	if err != nil {
		return "", fmt.Errorf("create OC session: %w", err)
	}

	a.mu.Lock()
	a.sessionID = session.ID
	a.mu.Unlock()

	return session.ID, nil
}

func (a *Adapter) ResumeSession(_ context.Context, sessionID string) error {
	a.mu.Lock()
	a.sessionID = sessionID
	a.mu.Unlock()
	return nil
}

func (a *Adapter) IsHealthy(ctx context.Context) bool {
	if a.proc == nil || !a.proc.isRunning() {
		return false
	}
	_, err := a.client.Session.List(ctx, oc.SessionListParams{})
	return err == nil
}

func (a *Adapter) sendEvent(ctx context.Context, evt runtime.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case a.events <- evt:
		return true
	}
}

// streamEvents connects to SSE and converts OC events to runtime.Event.
func (a *Adapter) streamEvents(ctx context.Context) {
	stream := a.client.Event.ListStreaming(ctx, oc.EventListParams{})
	defer stream.Close() //nolint:errcheck // best-effort cleanup

	for stream.Next() {
		if !a.processSSEEvent(ctx, stream.Current()) {
			return
		}
	}

	if err := stream.Err(); err != nil {
		log.Printf("[opencode] SSE stream error for %s: %v", a.cfg.AgentName, err)
		a.sendEvent(ctx, runtime.Event{
			Type:  runtime.EventError,
			Agent: a.cfg.AgentName,
			Text:  err.Error(),
		})
	}
}

func (a *Adapter) processSSEEvent(ctx context.Context, event oc.EventListResponse) bool {
	switch event.Type {
	case oc.EventListResponseTypeMessagePartUpdated:
		typed, ok := event.AsUnion().(oc.EventListResponseEventMessagePartUpdated)
		if ok && typed.Properties.Delta != "" {
			return a.sendEvent(ctx, runtime.Event{
				Type:  runtime.EventText,
				Agent: a.cfg.AgentName,
				Text:  typed.Properties.Delta,
			})
		}

	case oc.EventListResponseTypeSessionIdle:
		return a.sendEvent(ctx, runtime.Event{
			Type:  runtime.EventIdle,
			Agent: a.cfg.AgentName,
		})

	case oc.EventListResponseTypeSessionError:
		typed, ok := event.AsUnion().(oc.EventListResponseEventSessionError)
		if ok {
			return a.sendEvent(ctx, runtime.Event{
				Type:  runtime.EventError,
				Agent: a.cfg.AgentName,
				Text:  string(typed.Properties.Error.Name),
			})
		}

	default:
		// Handle question events not yet in the SDK typed constants.
		// OC emits "question.asked" SSE events for interactive questions.
		if string(event.Type) == "question.asked" {
			a.handleQuestionEvent(ctx, event)
		}
	}
	return true
}

// handleQuestionEvent parses a raw "question.asked" SSE event into EventQuestion.
func (a *Adapter) handleQuestionEvent(ctx context.Context, event oc.EventListResponse) {
	raw := event.JSON.RawJSON()

	var qEvent struct {
		Properties struct {
			RequestID string `json:"requestID"`
			Questions []struct {
				Question string `json:"question"`
				Header   string `json:"header"`
				Options  []struct {
					Label       string `json:"label"`
					Description string `json:"description"`
				} `json:"options"`
				Multiple bool `json:"multiple"`
				Custom   bool `json:"custom"`
			} `json:"questions"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(raw), &qEvent); err != nil {
		log.Printf("[opencode] failed to parse question event for %s: %v", a.cfg.AgentName, err)
		return
	}

	questions := make([]runtime.Question, 0, len(qEvent.Properties.Questions))
	for _, q := range qEvent.Properties.Questions {
		rq := runtime.Question{
			Header:      q.Header,
			Text:        q.Question,
			MultiSelect: q.Multiple,
			AllowCustom: q.Custom,
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
		a.sendEvent(ctx, runtime.Event{
			Type:          runtime.EventQuestion,
			Agent:         a.cfg.AgentName,
			CorrelationID: qEvent.Properties.RequestID,
			Questions:     questions,
		})
	}
}

// ReplyToQuestion sends a reply to a pending question via OC REST API.
func (a *Adapter) ReplyToQuestion(ctx context.Context, requestID string, answers []string) error {
	a.mu.Lock()
	sid := a.sessionID
	a.mu.Unlock()

	answerArrays := make([][]string, len(answers))
	for i, ans := range answers {
		answerArrays[i] = []string{ans}
	}

	replyURL := fmt.Sprintf("http://127.0.0.1:%d/session/%s/question/%s/reply", a.cfg.Port, sid, requestID)
	body, err := json.Marshal(map[string]interface{}{"answers": answerArrays})
	if err != nil {
		return fmt.Errorf("marshal OC question reply: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", replyURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("reply to OC question %s: %w", requestID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("OC question reply %s: status %d: %s", requestID, resp.StatusCode, string(respBody))
	}
	return nil
}
