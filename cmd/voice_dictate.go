//go:build voice_dictate && darwin

package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/dictate"
	"github.com/spf13/cobra"
)

var voiceDictateCmd = &cobra.Command{
	Use:   "dictate",
	Short: "Push-to-talk dictation (hold Right Option key)",
	Long: `Start push-to-talk dictation daemon.

Hold the Right Option key to record from your microphone.
Release to transcribe and paste into the focused app.

Prerequisites:
  brew install portaudio
  codesign -s - $(which ttal)

macOS permissions required (System Settings > Privacy & Security):
  - Input Monitoring (key listening)
  - Accessibility (paste simulation)
  - Microphone (audio recording)

Build with: go build -tags voice_dictate`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dictate.Run()
	},
}

func init() {
	voiceCmd.AddCommand(voiceDictateCmd)
}
