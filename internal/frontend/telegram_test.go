package frontend

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/addressee"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
	"github.com/tta-lab/ttal-cli/internal/telegram"
)

func TestTelegramSendText_AgentToHuman_UsesAgentBot(t *testing.T) {
	t.Setenv("YUKI_BOT_TOKEN", "yuki-bot-secret")
	var captured struct{ token, chatID, text string }
	sendMessageFn = func(token, chatID, text string) error {
		captured = struct{ token, chatID, text string }{token, chatID, text}
		return nil
	}
	t.Cleanup(func() { sendMessageFn = telegram.SendMessage })
	// Set up a temp team_path with a yuki agent so findAgent discovers it.
	tmp := t.TempDir()
	agentDir := tmp + "/yuki"
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(agentDir+"/AGENTS.md", []byte("---\nname: yuki\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := &TelegramFrontend{
		cfg: TelegramConfig{
			MCfg: &config.Config{
				TeamPath:          tmp,
				NotificationToken: "notify-secret",
				AdminHuman:        &humanfs.Human{TelegramChatID: "111"},
			},
		},
	}
	err := f.SendText(context.Background(),
		&addressee.Addressee{Kind: addressee.KindAgent, Name: "yuki"},
		&addressee.Addressee{Kind: addressee.KindHuman, Human: &humanfs.Human{Alias: "neil", TelegramChatID: "999"}},
		"hello",
	)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if captured.token != "yuki-bot-secret" {
		t.Errorf("token = %q, want yuki-bot-secret", captured.token)
	}
	if captured.chatID != "999" {
		t.Errorf("chatID = %q, want 999", captured.chatID)
	}
}

func TestTelegramSendText_NoFrom_UsesNotificationBot(t *testing.T) {
	var captured struct{ token, chatID string }
	sendMessageFn = func(token, chatID, _ string) error {
		captured = struct{ token, chatID string }{token, chatID}
		return nil
	}
	t.Cleanup(func() { sendMessageFn = telegram.SendMessage })
	f := &TelegramFrontend{
		cfg: TelegramConfig{
			MCfg: &config.Config{
				NotificationToken: "notify-secret",
				AdminHuman:        &humanfs.Human{TelegramChatID: "111"},
			},
		},
	}
	err := f.SendText(context.Background(), nil,
		&addressee.Addressee{Kind: addressee.KindHuman, Human: &humanfs.Human{TelegramChatID: "999"}},
		"system msg",
	)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if captured.token != "notify-secret" {
		t.Errorf("token = %q, want notify-secret", captured.token)
	}
}

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
