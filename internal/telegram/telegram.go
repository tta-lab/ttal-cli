package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ParseChatID converts a string chat ID to int64.
func ParseChatID(s string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid chat_id %q: %w", s, err)
	}
	return id, nil
}

// SendMessage sends a text message to a chat via the Telegram Bot API.
func SendMessage(botToken, chatID, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := bot.New(botToken)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	id, err := ParseChatID(chatID)
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

// SendVoice sends an OGG audio file as a voice message via Telegram Bot API.
func SendVoice(botToken string, chatID int64, oggData []byte) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("chat_id", fmt.Sprintf("%d", chatID)); err != nil {
		return err
	}

	part, err := writer.CreateFormFile("voice", "voice.ogg")
	if err != nil {
		return err
	}
	if _, err := part.Write(oggData); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendVoice", botToken)
	resp, err := http.Post(url, writer.FormDataContentType(), &buf) //nolint:gosec
	if err != nil {
		return fmt.Errorf("sendVoice: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendVoice returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ToolEmoji maps a tool name to a Telegram-allowed reaction emoji.
// Returns empty string for tools that should not trigger a reaction.
// Allowed list: https://gist.github.com/Soulter/3f22c8e5f9c7e152e967e8bc28c97fc9
func ToolEmoji(toolName string) string {
	switch toolName {
	case "Read", "Glob", "Grep":
		return "🤔"
	case "Edit", "Write":
		return "✍"
	case "Bash":
		return "👨‍💻"
	case "WebSearch", "WebFetch":
		return "👀"
	case "Agent":
		return "🔥"
	case "AskUserQuestion":
		return ""

	// CLI-specific refinements (from Bash input parsing)
	case "ttal:send", "ttal:route":
		return "🕊"
	case "flicknote:write":
		return "✍"
	case "flicknote:read":
		return "👀"

	default:
		return "🔥"
	}
}

// SetReaction sets an emoji reaction on a Telegram message.
// Setting a new reaction replaces the previous one (Telegram API behavior).
func SetReaction(botToken string, chatID int64, messageID int, emoji string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b, err := bot.New(botToken)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	_, err = b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    chatID,
		MessageID: messageID,
		Reaction: []models.ReactionType{
			{
				Type:              models.ReactionTypeTypeEmoji,
				ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: emoji},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("set reaction: %w", err)
	}
	return nil
}
