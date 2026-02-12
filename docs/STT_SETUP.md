# Speech-to-Text Setup

Local STT using Whisper on Apple Silicon via mlx-audio. Same infrastructure as the voice TTS server.

## Prerequisites

- mlx-audio already installed (see [VOICE_SETUP.md](VOICE_SETUP.md))
- Voice server running (`ttal voice status`)

## Model

Default: `mlx-community/whisper-large-v3-turbo` (~800MB, fast on Apple Silicon)

The model downloads automatically on first use.

## CLI Usage

### Basic transcription

```bash
mlx_audio.stt.generate \
  --model mlx-community/whisper-large-v3-turbo \
  --audio recording.wav \
  --output-path transcript.txt
```

### With vocabulary/hotwords

Use `--context` to improve recognition of domain-specific terms:

```bash
mlx_audio.stt.generate \
  --model mlx-community/whisper-large-v3-turbo \
  --audio recording.wav \
  --output-path transcript.txt \
  --context "ttal, Kokoro, OpenClaw, Yuki, Kestrel, Athena, zellij, clawd"
```

### With language hint

```bash
mlx_audio.stt.generate \
  --model mlx-community/whisper-large-v3-turbo \
  --audio recording.wav \
  --output-path transcript.txt \
  --language en
```

### Output formats

```bash
--format txt    # plain text (default)
--format srt    # subtitles
--format vtt    # web subtitles
--format json   # detailed with timestamps and segments
```

### Streaming (real-time output)

```bash
mlx_audio.stt.generate \
  --model mlx-community/whisper-large-v3-turbo \
  --audio recording.wav \
  --output-path transcript.txt \
  --stream
```

## API Usage

The voice server (`localhost:8877`) also exposes an OpenAI-compatible transcription endpoint:

```bash
curl -X POST http://localhost:8877/v1/audio/transcriptions \
  -F "file=@recording.wav" \
  -F "model=mlx-community/whisper-large-v3-turbo"
```

Response (JSON):

```json
{
  "text": "transcribed text here",
  "segments": [...],
  "language": "en"
}
```

Note: The realtime websocket endpoint (`/v1/audio/transcriptions/realtime`) requires `webrtcvad` which is patched out in our server. Use the POST endpoint or CLI instead.

## Model Options

| Model | Size | Speed | Accuracy |
|---|---|---|---|
| `mlx-community/whisper-large-v3-turbo` | ~800MB | Fast | Good |
| `mlx-community/whisper-large-v3` | ~1.5GB | Slower | Best |
| `mlx-community/whisper-small` | ~250MB | Fastest | Lower |

## Tips

- First run downloads the model (~800MB) — subsequent runs use cache
- For short audio (<30s), processing is near real-time on M-series chips
- The `--context` hotwords feature significantly improves accuracy for proper nouns and technical terms
- Supports common audio formats: wav, mp3, m4a, ogg, flac
