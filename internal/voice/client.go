package voice

import (
	"bytes"
	"fmt"
	"os/exec"
)

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
