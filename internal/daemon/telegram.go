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
	"os"
	"path/filepath"
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
	agentName string, cfg config.AgentConfig, chatID string, vocabulary []string,
	onMessage func(agentName, text string), done <-chan struct{},
) {
	go func() {
		backoff := 2 * time.Second

		for {
			select {
			case <-done:
				return
			default:
			}

			if err := runPoller(agentName, cfg, chatID, vocabulary, onMessage, done); err != nil {
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
	agentName string, cfg config.AgentConfig, effectiveChatID string, vocabulary []string,
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
		if update.Message.Chat.ID != chatID {
			return
		}
		if update.Message.From == nil {
			return
		}
		handleInboundMessage(ctx, b, update.Message, agentName, cfg.BotToken, effectiveChatID, vocabulary, onMessage)
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

func handleInboundMessage(
	ctx context.Context, b *bot.Bot, msg *models.Message,
	agentName, botToken, chatIDStr string, vocabulary []string,
	onMessage func(string, string),
) {
	senderName := msg.From.Username
	if senderName == "" {
		senderName = msg.From.FirstName
	}

	// Handle voice messages
	if msg.Voice != nil {
		transcription, err := transcribeVoiceMessage(ctx, b, msg.Voice, vocabulary)
		if err != nil {
			log.Printf("[telegram] voice transcription failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "Voice transcription failed — check daemon logs for details")
			return
		}
		formatted := formatInboundMessage(agentName, senderName, "[🎤 voice] "+transcription)
		onMessage(agentName, formatted)
		return
	}

	// Handle photo messages
	if len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		filename := fmt.Sprintf("photo_%d.jpg", msg.ID)

		localPath, err := downloadTelegramFile(ctx, b, photo.FileID, agentName, filename)
		if err != nil {
			log.Printf("[telegram] photo download failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "Photo download failed — check daemon logs for details")
			return
		}

		text := fmt.Sprintf("[📷 photo] %s", localPath)
		if caption := msg.Caption; caption != "" {
			text += " " + caption
		}
		onMessage(agentName, formatInboundMessage(agentName, senderName, text))
		return
	}

	// Handle document/file messages
	if msg.Document != nil {
		filename := msg.Document.FileName
		if filename == "" {
			filename = fmt.Sprintf("file_%d", msg.ID)
		}

		localPath, err := downloadTelegramFile(ctx, b, msg.Document.FileID, agentName, filename)
		if err != nil {
			log.Printf("[telegram] document download failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "File download failed — check daemon logs for details")
			return
		}

		text := fmt.Sprintf("[📎 file] %s", localPath)
		if caption := msg.Caption; caption != "" {
			text += " " + caption
		}
		onMessage(agentName, formatInboundMessage(agentName, senderName, text))
		return
	}

	text := strings.TrimSpace(msg.Text)
	onMessage(agentName, formatInboundMessage(agentName, senderName, text))
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
func transcribeVoiceMessage(ctx context.Context, b *bot.Bot, v *models.Voice, vocabulary []string) (string, error) {
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

	return sttTranscribe(audioData, "voice.ogg", vocabulary)
}

// sttTranscribe sends audio data to the mlx-audio STT endpoint and returns the transcribed text.
// vocabulary is joined into a context string that biases Whisper toward domain-specific terms.
func sttTranscribe(audioData []byte, filename string, vocabulary []string) (string, error) {
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
	if len(vocabulary) > 0 {
		hotwords := strings.Join(vocabulary, ", ")
		if err := writer.WriteField("context", hotwords); err != nil {
			return "", err
		}
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

// downloadTelegramFile downloads a file from Telegram and saves it to the agent's file directory.
// Returns the local file path.
func downloadTelegramFile(ctx context.Context, b *bot.Bot, fileID, agentName, filename string) (string, error) {
	// Sanitize filename to prevent path traversal
	filename = filepath.Base(filename)

	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}

	fileURL := b.FileDownloadLink(file)
	resp, err := http.Get(fileURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}

	dir := filepath.Join(config.ResolveDataDir(), "files", agentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create file dir: %w", err)
	}

	localPath := filepath.Join(dir, filename)

	// Avoid overwriting existing files — append timestamp
	if _, err := os.Stat(localPath); err == nil {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		localPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, time.Now().UnixMilli(), ext))
	}

	out, err := os.Create(localPath) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close file: %w", err)
	}

	return localPath, nil
}
