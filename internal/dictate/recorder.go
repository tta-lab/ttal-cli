//go:build voice_dictate && darwin

package dictate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"sync"

	"github.com/gordonklaus/portaudio"
)

const (
	sampleRate    = 16000 // 16kHz — what Whisper expects
	channels      = 1     // mono
	bitsPerSample = 16
)

// Recorder captures mic audio via PortAudio.
type Recorder struct {
	mu        sync.Mutex
	stream    *portaudio.Stream
	buf       []int16
	recording bool
}

// NewRecorder creates a Recorder. Call portaudio.Initialize() before use.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Start begins recording from the default mic.
func (r *Recorder) Start() error {
	r.mu.Lock()
	// Guard against double-Start leaking a stream
	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil
		r.recording = false
	}
	r.buf = r.buf[:0] // reset buffer, keep capacity
	r.mu.Unlock()

	processAudio := func(in []int16) {
		r.mu.Lock()
		r.buf = append(r.buf, in...)
		r.mu.Unlock()
	}

	stream, err := portaudio.OpenDefaultStream(channels, 0, float64(sampleRate), 0, processAudio)
	if err != nil {
		return fmt.Errorf("opening microphone: %w", err)
	}
	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("starting audio stream: %w", err)
	}

	r.mu.Lock()
	r.stream = stream
	r.recording = true
	r.mu.Unlock()
	return nil
}

// Stop ends recording and returns WAV-formatted audio bytes.
func (r *Recorder) Stop() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stream == nil {
		return nil, nil
	}

	if err := r.stream.Stop(); err != nil {
		log.Printf("[dictate] stream stop: %v", err)
	}
	if err := r.stream.Close(); err != nil {
		log.Printf("[dictate] stream close: %v", err)
	}
	r.stream = nil
	r.recording = false

	if len(r.buf) == 0 {
		return nil, nil
	}

	return pcmToWAV(r.buf), nil
}

// IsRecording returns whether the recorder is currently active.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// pcmToWAV wraps raw int16 PCM samples with a 44-byte WAV header.
func pcmToWAV(samples []int16) []byte {
	dataSize := len(samples) * 2 // 2 bytes per int16 sample
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	var buf bytes.Buffer
	buf.Grow(44 + dataSize)

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVEfmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16)) // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM format
	binary.Write(&buf, binary.LittleEndian, uint16(channels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	// Write PCM samples as little-endian int16
	binary.Write(&buf, binary.LittleEndian, samples)

	return buf.Bytes()
}
