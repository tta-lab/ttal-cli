package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// Speak generates speech audio and either plays it or saves to outputPath.
func Speak(text, voiceID string, speed float64, outputPath string) error {
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
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("voice server not reachable — check with: ttal voice status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("voice server error (%d): %s", resp.StatusCode, string(errBody))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read audio response: %w", err)
	}

	// Save to specified path
	if outputPath != "" {
		if err := os.WriteFile(outputPath, audioData, 0o644); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		fmt.Println(outputPath)
		return nil
	}

	// Auto-play and delete
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("ttal-voice-%d.wav", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, audioData, 0o644); err != nil {
		return fmt.Errorf("failed to write temp audio: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	cmd := exec.Command("afplay", tmpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
