package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
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

const maxMessageLen = 4096

// splitMessage splits text into chunks that fit within Telegram's 4096-rune limit.
// Splits at natural boundaries: paragraph breaks > newlines > spaces > hard cut.
func splitMessage(text string) []string {
	runes := []rune(text)
	if len(runes) <= maxMessageLen {
		return []string{text}
	}

	var parts []string
	for len(runes) > 0 {
		if len(runes) <= maxMessageLen {
			parts = append(parts, string(runes))
			break
		}

		chunk := string(runes[:maxMessageLen])
		cutAt := maxMessageLen // in runes
		if i := strings.LastIndex(chunk, "\n\n"); i > 0 {
			cutAt = len([]rune(chunk[:i]))
		} else if i := strings.LastIndex(chunk, "\n"); i > 0 {
			cutAt = len([]rune(chunk[:i]))
		} else if i := strings.LastIndex(chunk, " "); i > 0 {
			cutAt = len([]rune(chunk[:i]))
		}

		part := strings.TrimRight(string(runes[:cutAt]), " \n")
		if part != "" {
			parts = append(parts, part)
		}
		runes = []rune(strings.TrimLeft(string(runes[cutAt:]), " \n"))
	}
	return parts
}

// SendMessage sends a text message to a chat via the Telegram Bot API.
// Long messages are automatically split at natural boundaries to fit within Telegram's 4096-rune limit.
func SendMessage(botToken, chatID, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	b, err := bot.New(botToken)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	id, err := ParseChatID(chatID)
	if err != nil {
		return err
	}

	chunks := splitMessage(text)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(len(chunks)+1)*15*time.Second)
	defer cancel()

	for i, chunk := range chunks {
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: id,
			Text:   chunk,
		}); err != nil {
			return fmt.Errorf("telegram send (chunk %d/%d): %w", i+1, len(chunks), err)
		}
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
