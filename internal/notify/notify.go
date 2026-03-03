package notify

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/telegram"
)

// Send sends a notification to the team's Telegram chat using the notification bot.
// Loads config to resolve the active team's notification token and chat ID.
// Fire-and-forget: returns error but callers typically log and continue.
func Send(message string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.NotificationToken == "" {
		return fmt.Errorf("no notification bot token configured")
	}
	if cfg.ChatID == "" {
		return fmt.Errorf("no chat_id configured")
	}
	return telegram.SendMessage(cfg.NotificationToken, cfg.ChatID, message)
}

// SendWithConfig sends using pre-resolved token and chat ID (for daemon use).
// Silently returns nil if token or chatID are empty.
func SendWithConfig(token, chatID, message string) error {
	if token == "" || chatID == "" {
		return nil
	}
	return telegram.SendMessage(token, chatID, message)
}
