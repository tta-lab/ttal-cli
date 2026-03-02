package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tta-lab/ttal-cli/internal/config"
)

const (
	sttModel        = "mlx-community/whisper-large-v3-turbo-asr-fp16"
	sttEndpoint     = "http://localhost:8877/v1/audio/transcriptions"
	defaultLanguage = "en"
)

// Transcribe sends audio data to the mlx-audio STT endpoint.
// Reads vocabulary and language fresh from config on each call (hot-reload).
// Defaults to "en" when voice_language is unset; set to "auto" for Whisper auto-detect.
func Transcribe(audioData []byte, filename string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[voice] WARNING: config load failed, transcribing without vocabulary/language: %v", err)
		return transcribe(audioData, filename, nil, defaultLanguage)
	}
	// Merge agent names into vocabulary for Whisper accuracy.
	vocab := append([]string{}, cfg.Voice.Vocabulary...)
	for name := range cfg.Agents {
		vocab = append(vocab, name)
		if len(name) > 0 {
			r, size := utf8.DecodeRuneInString(name)
			capitalized := string(unicode.ToUpper(r)) + name[size:]
			if capitalized != name {
				vocab = append(vocab, capitalized)
			}
		}
	}

	lang := cfg.Voice.Language
	if lang == "" {
		lang = defaultLanguage
	}
	return transcribe(audioData, filename, vocab, lang)
}

func transcribe(audioData []byte, filename string, vocabulary []string, language string) (string, error) {
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
	// "auto" = omit language field, letting Whisper auto-detect.
	// Any other non-empty value is passed as an ISO 639-1 code.
	if language != "" && language != "auto" {
		if err := writer.WriteField("language", language); err != nil {
			return "", err
		}
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

	client := &http.Client{Timeout: 3 * time.Minute}
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
