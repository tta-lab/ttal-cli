package voice

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

const (
	plistName  = "io.guion.ttal.voice"
	serverPort = "8877"
	serverHost = "localhost"
	model      = "mlx-community/Kokoro-82M-bf16"
)

const serverScript = `#!/usr/bin/env python3
"""Minimal mlx-audio TTS server for ttal voice commands."""
import sys
import types

# Patch out webrtcvad (only needed for realtime STT, not TTS)
sys.modules["webrtcvad"] = types.ModuleType("webrtcvad")

from mlx_audio.server import app, model_provider  # noqa: E402
import uvicorn  # noqa: E402

# Pre-load the Kokoro model so first request is fast
model_provider.load_model("mlx-community/Kokoro-82M-bf16")

if __name__ == "__main__":
    uvicorn.run(app, host="localhost", port=8877)
`

func plistContent(pythonBin, scriptPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>%s</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <false/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>`, plistName, pythonBin, scriptPath, logPath, logPath)
}

// Install sets up the voice server script and launchd service.
func Install() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("voice server requires macOS (mlx-audio uses Apple Silicon)")
	}

	// Find mlx-audio's Python interpreter
	pythonBin, err := findMLXPython()
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	dataDir := config.ResolveDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// Write server script
	scriptPath := filepath.Join(dataDir, "voice-server.py")
	if err := os.WriteFile(scriptPath, []byte(serverScript), 0o755); err != nil {
		return fmt.Errorf("failed to write server script: %w", err)
	}
	fmt.Printf("Server script: %s\n", scriptPath)

	// Write launchd plist
	logPath := filepath.Join(dataDir, "voice-server.log")
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName+".plist")
	content := plistContent(pythonBin, scriptPath, logPath)
	if err := os.WriteFile(plistPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}
	fmt.Printf("Launchd plist: %s\n", plistPath)

	// Unload first (ignore errors if not loaded)
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	// Load the service
	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	fmt.Printf("Log file: %s\n", logPath)
	fmt.Println("\nVoice server installed and starting...")
	fmt.Println("Check status with: ttal voice status")
	return nil
}

// Uninstall removes the voice server launchd service and script.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Unload service
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName+".plist")
	if _, err := os.Stat(plistPath); err == nil {
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)
		fmt.Printf("Removed plist: %s\n", plistPath)
	} else {
		fmt.Println("Launchd plist: not installed")
	}

	// Remove server script
	scriptPath := filepath.Join(config.ResolveDataDir(), "voice-server.py")
	if _, err := os.Stat(scriptPath); err == nil {
		_ = os.Remove(scriptPath)
		fmt.Printf("Removed script: %s\n", scriptPath)
	}

	fmt.Println("\nVoice server uninstalled.")
	fmt.Println("Note: Log file remains at ~/.ttal/voice-server.log")
	return nil
}

// Status checks if the voice server is running and healthy.
func Status() error {
	url := fmt.Sprintf("http://%s:%s/v1/models", serverHost, serverPort)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Voice server: NOT RUNNING")
		fmt.Println("Start with: ttal voice install")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("Voice server: RUNNING")
		fmt.Printf("Endpoint: http://%s:%s\n", serverHost, serverPort)
	} else {
		fmt.Printf("Voice server: UNHEALTHY (status %d)\n", resp.StatusCode)
	}
	return nil
}

// findMLXPython locates the Python interpreter from the mlx-audio uv tool install.
func findMLXPython() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Check uv tool install location
	uvPython := filepath.Join(home, ".local", "share", "uv", "tools", "mlx-audio", "bin", "python3")
	if _, err := os.Stat(uvPython); err == nil {
		return uvPython, nil
	}

	return "", fmt.Errorf(
		"mlx-audio not found — install with:\n" +
			"  uv tool install mlx-audio --with \"misaki[en]\" --with uvicorn --with fastapi --with setuptools",
	)
}
