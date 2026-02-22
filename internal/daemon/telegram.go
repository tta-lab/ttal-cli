package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

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
	chatID, err := telegram.ParseChatID(effectiveChatID)
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

	defaultHandler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
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

		senderName := update.Message.From.Username
		if senderName == "" {
			senderName = update.Message.From.FirstName
		}

		// Handle voice messages
		if update.Message.Voice != nil {
			transcription, err := transcribeVoiceMessage(ctx, b, update.Message.Voice)
			if err != nil {
				log.Printf("[telegram] voice transcription failed for %s: %v", agentName, err)
				errMsg := "Voice transcription failed — check daemon logs for details"
				_ = telegram.SendMessage(cfg.BotToken, effectiveChatID, errMsg)
				return
			}
			formatted := formatInboundMessage(agentName, senderName, "[🎤 voice] "+transcription)
			onMessage(agentName, formatted)
			return
		}

		text := strings.TrimSpace(update.Message.Text)
		formatted := formatInboundMessage(agentName, senderName, text)
		onMessage(agentName, formatted)
	}

	b, err := bot.New(cfg.BotToken, bot.WithDefaultHandler(defaultHandler))
	if err != nil {
		return fmt.Errorf("bot init: %w", err)
	}

	// Register bot commands using the library's command matching.
	// MatchTypeCommandStartOnly uses Telegram's message entities to match
	// /command at the start of the message, avoiding false matches on
	// plain text like "new task..." that starts with a command word.
	registerBotCommands(b, agentName, cfg.BotToken, effectiveChatID, chatID)

	b.Start(ctx)
	return nil
}

// registerBotCommands registers each bot command as a handler on the bot instance.
// Uses RegisterHandlerMatchFunc with a custom matcher that:
//   - Checks for bot_command entity at message start (like MatchTypeCommandStartOnly)
//   - Strips @botname suffix from the command for group chat compatibility
//   - Validates chat ID so commands only work from the configured chat
func registerBotCommands(b *bot.Bot, agentName, botToken, chatIDStr string, chatID int64) {
	matchCommand := func(cmd string) bot.MatchFunc {
		return func(update *models.Update) bool {
			if update.Message == nil || update.Message.Chat.ID != chatID {
				return false
			}
			for _, e := range update.Message.Entities {
				if e.Type != models.MessageEntityTypeBotCommand || e.Offset != 0 {
					continue
				}
				// Extract command name: skip leading "/", strip @botname suffix
				raw := update.Message.Text[1:e.Length]
				name, _, _ := strings.Cut(raw, "@")
				if name == cmd {
					return true
				}
			}
			return false
		}
	}

	b.RegisterHandlerMatchFunc(matchCommand("status"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			args := parseCommandArgs(update.Message.Text)
			handleStatusCommand(agentName, botToken, chatIDStr, args)
		})

	b.RegisterHandlerMatchFunc(matchCommand("help"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			handleHelpCommand(botToken, chatIDStr)
		})

	b.RegisterHandlerMatchFunc(matchCommand("new"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			sendKeysToAgent(agentName, botToken, chatIDStr, "/new", "Sent /new — starting fresh conversation")
		})

	b.RegisterHandlerMatchFunc(matchCommand("compact"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			sendKeysToAgent(agentName, botToken, chatIDStr, "/compact", "Sent /compact — compacting conversation")
		})

	b.RegisterHandlerMatchFunc(matchCommand("wait"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			sendEscToAgent(agentName, botToken, chatIDStr)
		})
}

// parseCommandArgs extracts arguments after a /command from message text.
// e.g. "/status yuki" → ["yuki"]
func parseCommandArgs(text string) []string {
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

const (
	sttModel    = "mlx-community/whisper-large-v3-turbo-asr-fp16"
	sttEndpoint = "http://localhost:8877/v1/audio/transcriptions"
)

// transcribeVoiceMessage downloads a Telegram voice message and transcribes it via mlx-audio STT.
func transcribeVoiceMessage(ctx context.Context, b *bot.Bot, v *models.Voice) (string, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: v.FileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}

	fileURL := b.FileDownloadLink(file)
	resp, err := http.Get(fileURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("download voice: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download voice: HTTP %d", resp.StatusCode)
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read voice data: %w", err)
	}

	return sttTranscribe(audioData, "voice.ogg")
}

// sttTranscribe sends audio data to the mlx-audio STT endpoint and returns the transcribed text.
func sttTranscribe(audioData []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audioData); err != nil {
		return "", err
	}

	if err := writer.WriteField("model", sttModel); err != nil {
		return "", err
	}
	if err := writer.WriteField("language", "en"); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(sttEndpoint, writer.FormDataContentType(), &buf) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("STT request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("STT returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse STT response: %w", err)
	}

	return strings.TrimSpace(result.Text), nil
}
