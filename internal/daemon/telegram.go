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

		// Check for bot commands first (status, help, new, compact, wait)
		if handleBotCommand(agentName, cfg.BotToken, effectiveChatID, text) {
			return
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
