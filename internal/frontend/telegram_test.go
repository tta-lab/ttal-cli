package frontend

import (
	"context"
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
			want: "[telegram from:testuser] hello",
		},
		{
			name: "no space not bash mode",
			text: "!nospace",
			want: "[telegram from:testuser] !nospace",
		},
		{
			// TrimSpace strips the trailing space, so "! " → "!" which is NOT bash mode.
			name: "prefix only is not bash mode after trim",
			text: "! ",
			want: "[telegram from:testuser] !",
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
