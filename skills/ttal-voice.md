---
name: ttal-voice
description: Generate speech using your assigned Kokoro TTS voice.
---

# ttal voice

Generate speech using your assigned Kokoro TTS voice. Audio is generated locally on Apple Silicon via mlx-audio.

## Speak with your agent voice

```bash
ttal voice speak "<text>"
```

This looks up your registered voice from the agent database via the `TTAL_AGENT_NAME` env var (set automatically in team tmux sessions) and generates + plays the audio.

## Speak with a specific voice

```bash
ttal voice speak "<text>" --voice <voice_id>
```

## Save to file (for sending as voice note)

```bash
ttal voice speak "<text>" --output <path>
```

The output file is a `.wav`. Use this when you need to send audio as a voice message or attachment.

## Speed control

```bash
ttal voice speak "<text>" --speed 1.2
```

Speed range: 0.25 (slow) to 4.0 (fast). Default: 1.0.

## Check server status

```bash
ttal voice status
```

If the server is not running, tell the user to run `ttal voice install`.

## Available Voices

Use `ttal voice list` to see all voices. Common choices:

| Voice | Gender | Accent | Note |
|---|---|---|---|
| af_heart | Female | American | Warm, engaging |
| af_jessica | Female | American | Clear, direct |
| af_bella | Female | American | Youthful, soft |
| af_nova | Female | American | Professional |
| af_sky | Female | American | Bright, energetic |
| af_river | Female | American | Deeper, laid-back |
| af_sarah | Female | American | Calm, composed |
| am_adam | Male | American | Deep |
| am_eric | Male | American | |
| bf_emma | Female | British | Elegant |
| bm_george | Male | British | |

## When to Use Voice

- User asks you to "say" something aloud
- User requests a voice message or voice note
- You want to announce a completed task audibly
- Sending audio as an attachment (use `--output`)

## Notes

- The voice server must be running (`ttal voice status` to check)
- If the server is down, tell the user: "Run `ttal voice install` to start the voice server"
- Audio generation takes 2-4 seconds for typical sentences
- All processing is local — no API keys, no network, no cloud
