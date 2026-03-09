# Voice TTS Setup

ttal uses [Kokoro-82M](https://huggingface.co/hexgrad/Kokoro-82M) for text-to-speech via [mlx-audio](https://github.com/Blaizzy/mlx-audio), optimized for Apple Silicon with MLX framework.

## Requirements

- macOS with Apple Silicon (M1/M2/M3/M4)
- [uv](https://docs.astral.sh/uv/) package manager
- espeak-ng (system dependency for phonemizer)

## Installation

### 1. System dependency

```bash
brew install espeak-ng
```

### 2. Install mlx-audio with all TTS dependencies

```bash
uv tool install --force --python 3.12 \
  "mlx-audio[tts] @ git+https://github.com/Blaizzy/mlx-audio.git" \
  --prerelease=allow \
  --with "spacy>=3.8,<4"
```

### 3. Install pip and spacy English model into the tool's venv

```bash
uv pip install --python ~/.local/share/uv/tools/mlx-audio/bin/python3 pip
~/.local/share/uv/tools/mlx-audio/bin/python3 -m spacy download en_core_web_sm
```

### 4. Install server dependencies

```bash
uv pip install --python ~/.local/share/uv/tools/mlx-audio/bin/python3 \
  uvicorn fastapi python-multipart setuptools
```

### 5. (Optional) Set HuggingFace token for faster downloads

Create a read-only token at https://huggingface.co/settings/tokens, then:

```bash
huggingface-cli login
```

### 6. Install the ttal voice server

```bash
ttal voice install
```

This creates a launchd service (`io.guion.ttal.voice`) that runs the TTS server on `localhost:8877`.

## Usage

```bash
ttal voice status                              # check server health
ttal voice list                                # list 26 available voices
ttal voice speak "Hello world"                 # speak with default voice (af_heart)
ttal voice speak "Hello" --voice af_sky        # use specific voice
ttal voice speak "Hello"                       # use agent's assigned voice (via TTAL_AGENT_NAME env)
ttal voice speak "Hello" --output speech.wav   # save to file
```

### Assign voices to agents

```bash
ttal agent add yuki --voice af_sky +secretary
ttal agent modify yuki voice:af_nova
```

## Architecture

```
launchd (io.guion.ttal.voice)
  └── ~/.ttal/voice-server.py (Python, localhost:8877)
        └── mlx-audio FastAPI server
              └── Kokoro-82M-bf16 model (~160MB RAM)

ttal voice speak "text"  # TTAL_AGENT_NAME=yuki
  └── DB lookup → agent voice (via TTAL_AGENT_NAME env)
  └── POST localhost:8877/v1/audio/speech
  └── afplay (auto-play + delete temp file)
```

The server exposes an OpenAI-compatible `/v1/audio/speech` endpoint. The Go CLI is purely an HTTP client.

## The webrtcvad Hack

mlx-audio's server imports `webrtcvad` at module level for its realtime STT websocket feature. This dependency has a broken `pkg_resources` import on modern Python. Since we only use TTS (not STT), the voice server patches it out:

```python
import sys, types
sys.modules["webrtcvad"] = types.ModuleType("webrtcvad")
from mlx_audio.server import app  # now imports without error
```

This inserts a fake empty module into Python's import cache before `mlx_audio.server` is loaded. When the server tries `import webrtcvad`, Python finds the fake module and skips the real (broken) one. The TTS endpoint works perfectly — only the realtime STT websocket would fail if called.

## Troubleshooting

### Server not starting

Check the log:
```bash
tail -50 ~/.ttal/voice-server.log
```

### Missing module errors

If you see `No module named 'X'`, install it into the mlx-audio venv:
```bash
uv pip install --python ~/.local/share/uv/tools/mlx-audio/bin/python3 <module>
```

### Reinstalling from scratch

```bash
ttal voice uninstall
uv tool uninstall mlx-audio
# Then repeat steps 2-6 above
```

### Server management

```bash
# Manual stop/start
launchctl unload ~/Library/LaunchAgents/io.guion.ttal.voice.plist
launchctl load ~/Library/LaunchAgents/io.guion.ttal.voice.plist

# Full uninstall
ttal voice uninstall
```
