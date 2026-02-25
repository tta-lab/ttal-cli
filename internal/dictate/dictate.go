//go:build voice_dictate && darwin

package dictate

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/voice"
	"github.com/gordonklaus/portaudio"
)

// Run starts the dictation daemon. Blocks until Ctrl+C.
func Run() error {
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("portaudio init: %w (is portaudio installed? brew install portaudio)", err)
	}
	defer portaudio.Terminate()

	rec := NewRecorder()

	// recordStart is only accessed from onDown/onUp which both run on the
	// same CFRunLoop thread, so no synchronization is needed.
	var recordStart time.Time

	onDown := func() {
		recordStart = time.Now()
		if err := rec.Start(); err != nil {
			log.Printf("[dictate] record start failed: %v", err)
			return
		}
		fmt.Fprintf(os.Stderr, "\r[%s] Recording...", time.Now().Format("15:04:05"))
	}

	onUp := func() {
		duration := time.Since(recordStart)
		wavData, err := rec.Stop()
		if err != nil {
			log.Printf("[dictate] record stop failed: %v", err)
			return
		}
		if wavData == nil || duration < 300*time.Millisecond {
			fmt.Fprintf(os.Stderr, "\r[%s] Too short (%.1fs), skipped\n", time.Now().Format("15:04:05"), duration.Seconds())
			return
		}

		fmt.Fprintf(os.Stderr, "\r[%s] Recording... (%.1fs) — transcribing", time.Now().Format("15:04:05"), duration.Seconds())

		oggData, err := voice.ConvertWAVToOGG(wavData)
		if err != nil {
			log.Printf("\n[dictate] WAV→OGG conversion failed: %v", err)
			return
		}

		text, err := voice.Transcribe(oggData, "voice.ogg")
		if err != nil {
			log.Printf("\n[dictate] transcription failed: %v", err)
			return
		}

		if text == "" {
			fmt.Fprintf(os.Stderr, "\r[%s] (no speech detected)                    \n", time.Now().Format("15:04:05"))
			return
		}

		if err := PasteText(text); err != nil {
			log.Printf("\n[dictate] paste failed: %v", err)
			return
		}

		fmt.Fprintf(os.Stderr, "\r[%s] \"%s\"                    \n", time.Now().Format("15:04:05"), text)
	}

	fmt.Fprintln(os.Stderr, "Dictation active — hold Right Option to speak")
	fmt.Fprintln(os.Stderr, "   Server: http://localhost:8877")
	fmt.Fprintln(os.Stderr, "   Press Ctrl+C to stop")
	fmt.Fprintln(os.Stderr)

	// Handle Ctrl+C for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nStopping dictation...")
		rec.Stop() // stop active stream before terminating PortAudio
		portaudio.Terminate()
		os.Exit(0)
	}()

	// RunKeyTap blocks (runs CFRunLoop)
	return RunKeyTap(onDown, onUp)
}
