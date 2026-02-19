---
name: ttal
description: Agent infrastructure CLI. Manage PRs, send messages (Telegram/agent-to-agent), look up agent/project info, and generate spoken audio with your Kokoro voice.
metadata: { "openclaw": { "emoji": "🗣️" } }
---

# ttal — Agent Infrastructure CLI

ttal is the single interface for agent operations: PRs, messaging, agent/project registry, and voice.

## Voice (Text-to-Speech)

Generate speech using your assigned Kokoro TTS voice. Audio is generated locally on Apple Silicon via mlx-audio.

### Speak with your agent voice

```bash
ttal voice speak "<text>" --agent <your-agent-name>
```

This looks up your registered voice from the agent database and generates + plays the audio.

### Speak with a specific voice

```bash
ttal voice speak "<text>" --voice <voice_id>
```

### Save to file (for sending as voice note)

```bash
ttal voice speak "<text>" --agent <your-agent-name> --output <path>
```

The output file is a `.wav`. Use this when you need to send audio as a voice message or attachment.

### Speed control

```bash
ttal voice speak "<text>" --agent <your-agent-name> --speed 1.2
```

Speed range: 0.25 (slow) to 4.0 (fast). Default: 1.0.

### Check server status

```bash
ttal voice status
```

If the server is not running, tell the user to run `ttal voice install`.

## PR Management

Manage Forgejo pull requests from your worker session. Context is auto-resolved from your zellij session — no flags needed.

### Create a PR

```bash
ttal pr create "feat: add user authentication"
ttal pr create "fix: timeout bug" --body "Fixes #42"
```

Creates a PR using your task's branch. The PR index is stored in the task automatically.

### Modify a PR

```bash
ttal pr modify --title "updated title"
ttal pr modify --body "updated description"
```

### Merge a PR (squash)

```bash
ttal pr merge
ttal pr merge --keep-branch
```

Squash-merges the PR. Fails with a clear error if checks are failing or there are conflicts.

### Comment on a PR

```bash
ttal pr comment create "LGTM, ready to merge"
ttal pr comment list
```

### Override context

If not in a zellij worker session, provide the task UUID explicitly:

```bash
ttal pr create "title" --task <uuid>
```

## Messaging

### Telegram (automatic)

Your assistant text is automatically bridged to Telegram at the end of each turn via the CC Stop hook. No action needed — just reply naturally.

### Send to another agent (via Zellij)

```bash
ttal send --to <agent-name> "can you review my auth module?"
ttal send --from <your-name> --to <agent-name> "can you review my auth module?"
```

### Read message from stdin

```bash
echo "task complete" | ttal send --to <agent-name> --stdin
```

### Inbound message formats

Messages arrive as prefixed text in your input:

- `[telegram from:<name>]` — from a human via Telegram
- `[agent from:<name>]` — from another agent

### When to reply

- Meaningful updates: task complete, blocked, need input, PR ready
- Keep replies concise
- You don't need to reply to every message — use judgement

## Agent Info

Look up your own or another agent's details:

```bash
ttal agent info <name>
```

Returns: name, path, voice, tags, creation date, and matching projects.

## Project Info

Look up project details:

```bash
ttal project list              # list all active projects
ttal project info <alias>      # project details (path, repo, tags)
```

## Tag-Based Routing

Agents and projects share tags. An agent can see projects that share at least one tag:

```bash
ttal agent info yuki           # shows matching projects based on shared tags
ttal agent list +research      # list agents with the research tag
ttal project list +core        # list projects with the core tag
```

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
