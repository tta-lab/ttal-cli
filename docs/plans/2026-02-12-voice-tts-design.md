# Voice TTS Design

## Overview

Add text-to-speech support to ttal-cli using Kokoro-82M via mlx-audio. Agents can have assigned voices. A local mlx-audio server runs as a launchd service, and Go commands act as HTTP clients.

## Architecture

```
launchd (io.guion.ttal.voice)
  └── mlx-audio server (Python, localhost:8877)
        └── Kokoro-82M-bf16 (~160MB RAM)

ttal voice speak "Hello" --agent yuki
  └── DB lookup: yuki → voice: af_sky
  └── HTTP POST localhost:8877/v1/audio/speech
  └── afplay → delete temp file
```

## Schema Change

Add optional `voice` field to Agent:

```go
field.String("voice").
    Optional().
    Comment("Kokoro TTS voice ID (e.g. af_heart, af_sky)")
```

- `ttal agent modify yuki -- voice:af_sky` to set
- `ttal agent info yuki` shows voice if set
- Default fallback: `af_heart` when no voice is configured

## Voice Server

Tiny Python wrapper at `~/.ttal/voice-server.py`:
- Patches `webrtcvad` out of `sys.modules` (only needed for STT)
- Starts mlx-audio FastAPI on `localhost:8877`
- Pre-loads `mlx-community/Kokoro-82M-bf16`

launchd plist `io.guion.ttal.voice`:
- Uses mlx-audio uv tool's Python interpreter
- `KeepAlive: true` (auto-restart on crash)
- Logs to `~/.ttal/voice-server.log`
- `RunAtLoad: false`

## Commands

```
ttal voice install          # write server script + plist, load service
ttal voice uninstall        # unload service, remove plist + script
ttal voice status           # ping /v1/models health endpoint
ttal voice speak "text"     # generate + auto-play, delete temp file
  --voice af_sky            # override voice
  --agent yuki              # look up voice from agent's DB record
  --output /path/out.wav    # save to file instead of auto-play
  --speed 1.0               # speech speed (0.25-4.0)
ttal voice list             # print available Kokoro voice IDs
```

Voice priority: `--voice` flag > `--agent` DB lookup > default `af_heart`.

## File Layout

```
cmd/voice.go                  # cobra commands
internal/voice/
  install.go                  # install/uninstall/status
  client.go                   # HTTP client for /v1/audio/speech
  voices.go                   # hardcoded voice list
```

## Prerequisites

- `uv tool install mlx-audio --with "misaki[en]"` (user installs once)
- macOS with Apple Silicon (MLX framework requirement)
