package daemon

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// sendTelegramMessage sends a text message to the configured chat via the bot.
func sendTelegramMessage(botToken, chatID, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := bot.New(botToken)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	id, err := parseChatID(chatID)
	if err != nil {
		return err
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: id,
		Text:   text,
	}); err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}

	return nil
}

// startTelegramPoller starts a long-poll loop for one agent's bot.
// It calls onMessage for each new user message, formatted for CC delivery.
// Runs until done is closed.
func startTelegramPoller(
	agentName string, cfg config.AgentConfig, chatID string, onMessage func(agentName, text string), done <-chan struct{},
) {
	go func() {
		backoff := 2 * time.Second

		for {
			select {
			case <-done:
				return
			default:
			}

			if err := runPoller(agentName, cfg, chatID, onMessage, done); err != nil {
				log.Printf("[telegram] poller for %s failed: %v — retrying in %s", agentName, err, backoff)
				select {
				case <-done:
					return
				case <-time.After(backoff):
				}
				if backoff < 5*time.Minute {
					backoff *= 2
				}
			} else {
				backoff = 2 * time.Second
			}
		}
	}()
}

func runPoller(
	agentName string, cfg config.AgentConfig, effectiveChatID string,
	onMessage func(agentName, text string), done <-chan struct{},
) error {
	chatID, err := parseChatID(effectiveChatID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel context when done is closed
	go func() {
		<-done
		cancel()
	}()

	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		// Only accept messages from the configured chat
		if update.Message.Chat.ID != chatID {
			return
		}
		// From is nil for channel posts
		if update.Message.From == nil {
			return
		}

		text := strings.TrimSpace(update.Message.Text)

		// Check for bot commands first (status, help, new, compact, wait)
		if handleBotCommand(agentName, cfg.BotToken, effectiveChatID, text) {
			return
		}

		senderName := update.Message.From.Username
		if senderName == "" {
			senderName = update.Message.From.FirstName
		}

		formatted := formatInboundMessage(agentName, senderName, text)
		onMessage(agentName, formatted)
	}

	b, err := bot.New(cfg.BotToken, bot.WithDefaultHandler(handler))
	if err != nil {
		return fmt.Errorf("bot init: %w", err)
	}

	b.Start(ctx)
	return nil
}

// parseChatID converts a string chat ID to int64.
func parseChatID(s string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid chat_id %q: %w", s, err)
	}
	return id, nil
}
