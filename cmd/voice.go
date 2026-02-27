package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
	"codeberg.org/clawteam/ttal-cli/internal/voice"
	"github.com/spf13/cobra"
)

var (
	speakVoice string
	speakSpeed float64
)

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Text-to-speech using Kokoro TTS",
	Long:  `Generate speech audio using a local Kokoro TTS server. Supports per-agent voice assignment.`,
}

var voiceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install voice server as launchd service",
	Long: `Set up the mlx-audio TTS server as a background service:

1. Writes server script to ~/.ttal/voice-server.py
2. Creates launchd plist (io.guion.ttal.voice)
3. Loads and starts the service

Prerequisites:
  uv tool install mlx-audio --with "misaki[en]" --with uvicorn --with fastapi --with setuptools`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return voice.Install()
	},
}

var voiceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove voice server service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return voice.Uninstall()
	},
}

var voiceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check voice server status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return voice.Status()
	},
}

var voiceSpeakCmd = &cobra.Command{
	Use:   `speak "text to speak"`,
	Short: "Generate speech and send as Telegram voice message",
	Long: `Convert text to speech and send as a Telegram voice bubble.

Requires TTAL_AGENT_NAME env var to resolve bot token and chat ID.
Voice priority: --voice flag > TTAL_AGENT_NAME DB lookup > default (af_heart)

Examples:
  ttal voice speak "Hello world"
  ttal voice speak "Good morning" --voice af_nova`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		voiceID := speakVoice
		agentName := os.Getenv("TTAL_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("TTAL_AGENT_NAME is required to send voice via Telegram")
		}

		// Look up agent voice from DB if --voice is not set
		if voiceID == "" {
			ctx := context.Background()
			ag, err := database.Agent.Query().
				Where(agent.Name(strings.ToLower(agentName))).
				Only(ctx)
			if err != nil {
				return fmt.Errorf("agent '%s' not found", agentName)
			}
			voiceID = ag.Voice
		}

		if voiceID != "" && !voice.IsValidVoice(voiceID) {
			return fmt.Errorf("unknown voice '%s' — run 'ttal voice list' to see available voices", voiceID)
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		agentCfg, ok := cfg.Agents[agentName]
		if !ok {
			return fmt.Errorf("agent %s not found in config", agentName)
		}

		wavData, err := voice.SpeakToBytes(text, voiceID, speakSpeed)
		if err != nil {
			return err
		}

		oggData, err := voice.ConvertWAVToOGG(wavData)
		if err != nil {
			return err
		}

		chatIDStr := cfg.ChatID
		chatID, err := telegram.ParseChatID(chatIDStr)
		if err != nil {
			return err
		}

		return telegram.SendVoice(agentCfg.BotToken, chatID, oggData)
	},
}

var voiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available voices",
	RunE: func(cmd *cobra.Command, args []string) error {
		voice.PrintVoiceList()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(voiceCmd)

	voiceCmd.AddCommand(voiceInstallCmd)
	voiceCmd.AddCommand(voiceUninstallCmd)
	voiceCmd.AddCommand(voiceStatusCmd)
	voiceCmd.AddCommand(voiceSpeakCmd)
	voiceCmd.AddCommand(voiceListCmd)

	voiceSpeakCmd.Flags().StringVar(&speakVoice, "voice", "", "Voice ID (e.g. af_heart, af_sky)")
	voiceSpeakCmd.Flags().Float64Var(&speakSpeed, "speed", 1.0, "Speech speed (0.25-4.0)")
}
