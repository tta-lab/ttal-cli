package codex

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tta-lab/codex-server-go/protocol"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestProcessNotificationItemStarted(t *testing.T) {
	tests := []struct {
		name     string
		params   string
		wantTool string
	}{
		{
			name: "commandExecution",
			params: `{"item":{"type":"commandExecution","id":"i1","command":"ls",` +
				`"cwd":"/tmp","status":"inProgress","commandActions":[]},"threadId":"t1","turnId":"r1"}`,
			wantTool: "Bash",
		},
		{
			name:     "fileChange",
			params:   `{"item":{"type":"fileChange","id":"i2"},"threadId":"t1","turnId":"r1"}`,
			wantTool: "Edit",
		},
		{
			name:     "mcpToolCall",
			params:   `{"item":{"type":"mcpToolCall","id":"i3"},"threadId":"t1","turnId":"r1"}`,
			wantTool: "WebSearch",
		},
		{
			name:     "webSearch",
			params:   `{"item":{"type":"webSearch","id":"i4"},"threadId":"t1","turnId":"r1"}`,
			wantTool: "WebSearch",
		},
		{
			name:     "reasoning",
			params:   `{"item":{"type":"reasoning","id":"i5","summary":[]},"threadId":"t1","turnId":"r1"}`,
			wantTool: "Read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				cfg:    runtime.AdapterConfig{AgentName: "test"},
				events: make(chan runtime.Event, 16),
			}

			a.processNotification(rpcResponse{
				Method: protocol.NotifItemStarted,
				Params: json.RawMessage(tt.params),
			})

			select {
			case evt := <-a.events:
				if evt.Type != runtime.EventTool {
					t.Errorf("got event type %q, want %q", evt.Type, runtime.EventTool)
				}
				if evt.ToolName != tt.wantTool {
					t.Errorf("got tool name %q, want %q", evt.ToolName, tt.wantTool)
				}
			default:
				t.Error("no event emitted")
			}
		})
	}
}

func TestProcessNotificationItemStartedNoEvent(t *testing.T) {
	a := &Adapter{
		cfg:    runtime.AdapterConfig{AgentName: "test"},
		events: make(chan runtime.Event, 16),
	}

	a.processNotification(rpcResponse{
		Method: protocol.NotifItemStarted,
		Params: json.RawMessage(`{"item":{"type":"contextCompaction","id":"i6"},"threadId":"t1","turnId":"r1"}`),
	})

	select {
	case evt := <-a.events:
		t.Errorf("expected no event for contextCompaction, got %+v", evt)
	default:
		// expected — no event emitted
	}
}

func TestCodexItemToToolName(t *testing.T) {
	tests := []struct {
		itemType string
		want     string
	}{
		{"commandExecution", "Bash"},
		{"fileChange", "Edit"},
		{"webSearch", "WebSearch"},
		{"imageView", "WebSearch"},
		{"openPage", "WebSearch"},
		{"mcpToolCall", "WebSearch"},
		{"reasoning", "Read"},
		{"plan", "Read"},
		{"search", "Read"},
		{"findInPage", "Read"},
		{"collabAgentToolCall", "Agent"},
		{"contextCompaction", ""},
		{"unknownType", "unknownType"},
	}

	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			got := codexItemToToolName(tt.itemType)
			if got != tt.want {
				t.Errorf("codexItemToToolName(%q) = %q, want %q", tt.itemType, got, tt.want)
			}
		})
	}
}

func TestProcessNotificationTokenUsageUpdated(t *testing.T) {
	a := &Adapter{
		cfg:    runtime.AdapterConfig{AgentName: "test"},
		events: make(chan runtime.Event, 16),
	}
	contextWindow := int64(128000)
	//nolint:lll
	params := fmt.Sprintf(
		`{"threadId":"t1","turnId":"r1","tokenUsage":{"total":{"totalTokens":64000,"inputTokens":50000,"outputTokens":14000,"cachedInputTokens":0,"reasoningOutputTokens":0},"last":{"totalTokens":1000,"inputTokens":800,"outputTokens":200,"cachedInputTokens":0,"reasoningOutputTokens":0},"modelContextWindow":%d}}`,
		contextWindow,
	)
	a.processNotification(rpcResponse{
		Method: protocol.NotifThreadTokenUsageUpdated,
		Params: json.RawMessage(params),
	})
	evt := <-a.events
	if evt.Type != runtime.EventStatus {
		t.Fatalf("want EventStatus, got %s", evt.Type)
	}
	if evt.ContextUsedPct != 50.0 {
		t.Errorf("want 50%%, got %.1f%%", evt.ContextUsedPct)
	}
	if evt.ContextRemainingPct != 50.0 {
		t.Errorf("want 50%% remaining, got %.1f%%", evt.ContextRemainingPct)
	}
}

func TestCreateSessionV2(t *testing.T) {
	// Verify the response parsing logic for thread/start v2 format
	result := json.RawMessage(
		`{"thread":{"id":"test-thread-id","turns":[]},"model":"gpt-4.1",` +
			`"modelProvider":"openai","cwd":"/tmp","approvalPolicy":"never","sandbox":"none"}`,
	)

	var resp struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("failed to parse v2 response: %v", err)
	}
	if resp.Thread.ID != "test-thread-id" {
		t.Errorf("got thread ID %q, want %q", resp.Thread.ID, "test-thread-id")
	}
}
