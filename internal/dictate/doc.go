// Package dictate implements a push-to-talk voice dictation daemon for macOS.
//
// Holding the Right Option key starts audio recording via PortAudio; releasing it
// converts the captured WAV data to OGG, sends it to the Whisper transcription
// server, and pastes the resulting text at the current cursor position using
// macOS accessibility APIs. Built only when the voice_dictate build tag is set
// and requires portaudio and ffmpeg to be installed.
//
// Plane: manager
package dictate
