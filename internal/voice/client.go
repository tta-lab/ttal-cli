package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed"`
	LangCode       string  `json:"lang_code"`
	ResponseFormat string  `json:"response_format"`
}

// SpeakToBytes generates TTS audio and returns WAV bytes.
func SpeakToBytes(text, voiceID string, speed float64) ([]byte, error) {
	if voiceID == "" {
		voiceID = DefaultVoice
	}

	url := fmt.Sprintf("http://%s:%s/v1/audio/speech", serverHost, serverPort)

	reqBody := SpeechRequest{
		Model:          model,
		Input:          text,
		Voice:          voiceID,
		Speed:          speed,
		LangCode:       "a",
		ResponseFormat: "wav",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("voice server not reachable — check with: ttal voice status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("voice server error (%d): %s", resp.StatusCode, string(errBody))
	}

	return io.ReadAll(resp.Body)
}

// ConvertWAVToOGG converts WAV audio bytes to OGG/Opus format via ffmpeg.
func ConvertWAVToOGG(wavData []byte) ([]byte, error) {
	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-c:a", "libopus", "-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(wavData)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg convert: %w: %s", err, stderr.String())
	}
	return out.Bytes(), nil
}
