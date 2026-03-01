---
title: Voice
description: Local TTS and STT setup for ttal
---

ttal supports two-way voice communication with your agents. Text-to-speech (TTS) lets agents speak back with per-agent Kokoro voices. Speech-to-text (STT) transcribes incoming Telegram voice messages.

Both features run locally on Apple Silicon — no cloud API keys required.

## Requirements

- macOS with Apple Silicon (M1 or later)
- Python package manager (`uv` recommended)
- `espeak-ng` (for phoneme generation)

## Installation

### 1. Install mlx-audio

```bash
uv tool install mlx-audio[tts]
```

Install the spaCy language model:

```bash
uv tool run --from mlx-audio python -m spacy download en_core_web_sm
```

### 2. Install the voice service

```bash
ttal voice install
```

This creates a launchd plist that runs the mlx-audio server on `localhost:8877`.

### 3. Install espeak-ng

```bash
brew install espeak-ng
```

## Text-to-speech (TTS)

Agents can speak back using Kokoro voices:

```bash
# Speak with default voice
ttal voice speak "The deployment is complete."

# Speak with a specific voice
ttal voice speak "Hello from Athena." --voice af_heart

# Save to file
ttal voice speak "Test message" --output speech.wav
```

### Per-agent voices

Assign voices when adding agents:

```bash
ttal agent add athena --voice af_heart +research
```

List available voices:

```bash
ttal voice list
```

There are 26 Kokoro voices available — different pitches, accents, and styles.

When an agent responds via Telegram, ttal can render the response as a voice message using the agent's assigned voice.

## Speech-to-text (STT)

Incoming Telegram voice messages are automatically transcribed using Whisper (via mlx-audio).

The default model is `mlx-community/whisper-large-v3-turbo` (~800MB, downloaded on first use).

### Configuration

Set the voice language in your config to improve accuracy:

```toml
[teams.default]
voice_language = "en"    # ISO 639-1 code, or "auto" for auto-detection
```

Add custom vocabulary for domain-specific terms:

```toml
[teams.default]
voice_vocabulary = ["ttal", "taskwarrior", "kokoro", "forgejo"]
```

This helps Whisper correctly transcribe technical terms that it might otherwise mishear.

## How it works

The mlx-audio server exposes an OpenAI-compatible API:

- **TTS**: `POST /v1/audio/speech` — generates audio from text
- **STT**: `POST /v1/audio/transcriptions` — transcribes audio to text

Both run entirely on-device using Apple's MLX framework. No data leaves your machine.
