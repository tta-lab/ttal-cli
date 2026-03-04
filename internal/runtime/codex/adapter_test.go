package codex

import (
	"encoding/json"
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
			wantTool: "MCP",
		},
		{
			name:     "webSearch",
			params:   `{"item":{"type":"webSearch","id":"i4"},"threadId":"t1","turnId":"r1"}`,
			wantTool: "WebSearch",
		},
		{
			name:     "reasoning",
			params:   `{"item":{"type":"reasoning","id":"i5","summary":[]},"threadId":"t1","turnId":"r1"}`,
			wantTool: "reasoning",
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

func TestCodexItemToToolName(t *testing.T) {
	tests := []struct {
		itemType string
		want     string
	}{
		{"commandExecution", "Bash"},
		{"fileChange", "Edit"},
		{"webSearch", "WebSearch"},
		{"mcpToolCall", "MCP"},
		{"reasoning", "reasoning"},
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

func TestProcessServerRequestUserInput(t *testing.T) {
	a := &Adapter{
		cfg:    runtime.AdapterConfig{AgentName: "test"},
		events: make(chan runtime.Event, 16),
	}

	a.processServerRequest(rpcResponse{
		ID:     json.RawMessage(`42`),
		Method: protocol.ReqItemToolRequestUserInput,
		Params: json.RawMessage(`{
			"threadId": "t1",
			"turnId": "r1",
			"itemId": "item1",
			"questions": [{
				"id": "q1",
				"header": "Approve?",
				"question": "Allow this action?",
				"isOther": true,
				"isSecret": false,
				"options": [{"label": "Yes", "description": "Allow"}, {"label": "No", "description": "Deny"}]
			}]
		}`),
	})

	select {
	case evt := <-a.events:
		if evt.Type != runtime.EventQuestion {
			t.Fatalf("got event type %q, want %q", evt.Type, runtime.EventQuestion)
		}
		if evt.CorrelationID != "42" {
			t.Errorf("got correlationID %q, want %q", evt.CorrelationID, "42")
		}
		if len(evt.Questions) != 1 {
			t.Fatalf("got %d questions, want 1", len(evt.Questions))
		}
		q := evt.Questions[0]
		if q.ID != "q1" || q.Header != "Approve?" || q.Text != "Allow this action?" {
			t.Errorf("unexpected question fields: %+v", q)
		}
		if !q.AllowCustom {
			t.Error("expected AllowCustom to be true")
		}
		if len(q.Options) != 2 {
			t.Errorf("got %d options, want 2", len(q.Options))
		}
	default:
		t.Error("no event emitted")
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
