package frontend

import (
	"context"
	"os"
	"testing"

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
