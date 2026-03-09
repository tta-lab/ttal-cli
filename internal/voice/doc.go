// Package voice provides text-to-speech generation via a local Kokoro-compatible server.
//
// It sends TTS requests to a locally running speech server, receives WAV audio,
// and converts it to OGG/Opus format via ffmpeg for delivery as Telegram voice
// messages. It also maintains the list of available voice IDs and their metadata.
//
// Plane: shared
package voice
