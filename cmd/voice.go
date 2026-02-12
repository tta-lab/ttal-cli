package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/guion-opensource/ttal-cli/ent/agent"
	"github.com/guion-opensource/ttal-cli/internal/voice"
	"github.com/spf13/cobra"
)

var (
	speakVoice  string
	speakAgent  string
	speakOutput string
	speakSpeed  float64
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
	Short: "Generate and play speech",
	Long: `Convert text to speech and play it.

Voice priority: --voice flag > --agent DB lookup > default (af_heart)

Examples:
  ttal voice speak "Hello world"
  ttal voice speak "Good morning" --voice af_nova
  ttal voice speak "Task complete" --agent yuki
  ttal voice speak "Save this" --output ~/speech.wav`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		voiceID := speakVoice

		// Look up agent voice if --agent is set and --voice is not
		if voiceID == "" && speakAgent != "" {
			ctx := context.Background()
			ag, err := database.Agent.Query().
				Where(agent.Name(strings.ToLower(speakAgent))).
				Only(ctx)
			if err != nil {
				return fmt.Errorf("agent '%s' not found", speakAgent)
			}
			voiceID = ag.Voice
		}

		if voiceID != "" && !voice.IsValidVoice(voiceID) {
			return fmt.Errorf("unknown voice '%s' — run 'ttal voice list' to see available voices", voiceID)
		}

		return voice.Speak(text, voiceID, speakSpeed, speakOutput)
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
	voiceSpeakCmd.Flags().StringVar(&speakAgent, "agent", "", "Look up voice from agent record")
	voiceSpeakCmd.Flags().StringVar(&speakOutput, "output", "", "Save audio to file instead of playing")
	voiceSpeakCmd.Flags().Float64Var(&speakSpeed, "speed", 1.0, "Speech speed (0.25-4.0)")
}
