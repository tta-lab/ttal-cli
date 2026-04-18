package frontend

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestBuildFullCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmdName     string
		messageText string
		want        string
	}{
		{
			name:        "command with no args",
			cmdName:     "triage",
			messageText: "/triage",
			want:        "/triage",
		},
		{
			name:        "command with args",
			cmdName:     "triage",
			messageText: "/triage fix this",
			want:        "/triage fix this",
		},
		{
			name:        "command with multiple args",
			cmdName:     "brainstorm",
			messageText: "/brainstorm new feature ideas",
			want:        "/brainstorm new feature ideas",
		},
		{
			name:        "hyphenated skill name with args",
			cmdName:     "write-plan",
			messageText: "/write_plan implement auth",
			want:        "/write-plan implement auth",
		},
		{
			name:        "command with @bot suffix in group chat",
			cmdName:     "write-plan",
			messageText: "/write_plan@somebot implement auth",
			want:        "/write-plan implement auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSlashCommand(tt.cmdName, tt.messageText)
			if got != tt.want {
				t.Errorf("buildSlashCommand(%q, %q) = %q, want %q", tt.cmdName, tt.messageText, got, tt.want)
			}
		})
	}
}

// TestHandleInboundMessage_BashMode verifies that messages starting with "! " are delivered
// directly to the agent without the [telegram from:] wrapper.
func TestHandleInboundMessage_BashMode(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		overrideText *string
		replyTo      *models.Message
		want         string
	}{
		{
			name: "bash mode",
			text: "! ls",
			want: "! ls",
		},
		{
			name: "normal text",
			text: "hello",
			want: "[telegram from:testuser] hello\n\n<i>--- Reply with: ttal send --to human \"your message\"</i>",
		},
		{
			name: "no space not bash mode",
			text: "!nospace",
			want: "[telegram from:testuser] !nospace\n\n<i>--- Reply with: ttal send --to human \"your message\"</i>",
		},
		{
			// TrimSpace strips the trailing space, so "! " → "!" which is NOT bash mode.
			name: "prefix only is not bash mode after trim",
			text: "! ",
			want: "[telegram from:testuser] !\n\n<i>--- Reply with: ttal send --to human \"your message\"</i>",
		},
		{
			name:         "overrideText bash mode",
			text:         "@bot ! ls",
			overrideText: strPtr("! ls"),
			want:         "! ls",
		},
		{
			name:    "reply context dropped for bash mode",
			text:    "! ls",
			replyTo: &models.Message{Text: "previous message"},
			want:    "! ls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := &TelegramFrontend{
				cfg: TelegramConfig{
					UserNameFn: func() string { return "testuser" },
				},
				mt: newMessageTracker(),
			}

			var got string
			onMessage := func(agentName, text string) { got = text }

			msg := &models.Message{
				ID:             1,
				Text:           tt.text,
				ReplyToMessage: tt.replyTo,
				From:           &models.User{Username: "testuser"},
				Chat:           models.Chat{ID: 123},
			}

			fe.handleInboundMessage(
				context.Background(), nil, msg,
				"testteam", "testagent", "test-token", "123",
				onMessage, tt.overrideText,
			)

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// TestHandleInboundMessage_NormalIncludesHint verifies that normal (non-bash) inbound
// messages include the italic reply hint footer.
func TestHandleInboundMessage_NormalIncludesHint(t *testing.T) {
	fe := &TelegramFrontend{
		cfg: TelegramConfig{
			UserNameFn: func() string { return "neil" },
		},
		mt: newMessageTracker(),
	}

	var got string
	onMessage := func(agentName, text string) { got = text }

	msg := &models.Message{
		ID:   1,
		Text: "hello there",
		From: &models.User{Username: "neil"},
		Chat: models.Chat{ID: 123},
	}

	fe.handleInboundMessage(
		context.Background(), nil, msg,
		"default", "yuki", "token", "123",
		onMessage, nil,
	)

	hint := "<i>--- Reply with: ttal send --to human \"your message\"</i>"
	if !strings.Contains(got, "[telegram from:neil] hello there") {
		t.Errorf("expected prefix missing, got %q", got)
	}
	if !strings.Contains(got, hint) {
		t.Errorf("expected italic hint %q missing from %q", hint, got)
	}
}

// TestHandleInboundMessage_BashModeNoHint verifies that bash-mode messages do NOT
// include the reply hint footer.
func TestHandleInboundMessage_BashModeNoHint(t *testing.T) {
	fe := &TelegramFrontend{
		cfg: TelegramConfig{
			UserNameFn: func() string { return "neil" },
		},
		mt: newMessageTracker(),
	}

	var got string
	onMessage := func(agentName, text string) { got = text }

	msg := &models.Message{
		ID:   1,
		Text: "! ls /tmp",
		From: &models.User{Username: "neil"},
		Chat: models.Chat{ID: 123},
	}

	fe.handleInboundMessage(
		context.Background(), nil, msg,
		"default", "yuki", "token", "123",
		onMessage, nil,
	)

	if got != "! ls /tmp" {
		t.Errorf("got %q, want %q", got, "! ls /tmp")
	}
	if strings.Contains(got, "ttal send") {
		t.Errorf("bash mode should not contain hint, got %q", got)
	}
}
