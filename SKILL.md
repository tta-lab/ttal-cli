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
ttal voice speak "<text>"
```

This looks up your registered voice from the agent database via the `TTAL_AGENT_NAME` env var (set automatically in team tmux sessions) and generates + plays the audio.

### Speak with a specific voice

```bash
ttal voice speak "<text>" --voice <voice_id>
```

### Save to file (for sending as voice note)

```bash
ttal voice speak "<text>" --output <path>
```

The output file is a `.wav`. Use this when you need to send audio as a voice message or attachment.

### Speed control

```bash
ttal voice speak "<text>" --speed 1.2
```

Speed range: 0.25 (slow) to 4.0 (fast). Default: 1.0.

### Check server status

```bash
ttal voice status
```

If the server is not running, tell the user to run `ttal voice install`.

## PR Management

Manage Forgejo pull requests from your worker session. Context is auto-resolved from TTAL_JOB_ID — no flags needed.

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
ttal pr comment create "LGTM — no critical issues"
ttal pr comment list
```

## Messaging

### Send to another agent

```bash
ttal send --to <agent-name> "can you review my auth module?"
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

Returns: name, path, voice, role, and description.

## Project Info

Look up project details:

```bash
ttal project list              # list all active projects
ttal project info <alias>      # project details (path, repo)
```

## Today Focus

Manage your daily task focus list (uses taskwarrior `scheduled` date):

```bash
ttal today list                # pending tasks scheduled on or before today
ttal today completed           # tasks completed today
ttal today add <uuid>          # set scheduled:today on a task
ttal today remove <uuid>       # clear scheduled date from a task
```

## Task Management

Create tasks and export rich prompts for piping to agents:

```bash
# Create a task (project is required, validated against ttal project DB)
ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"

# Tags and annotations are repeatable
ttal task add --project ttal "Fix auth bug" --tag bugfix --tag urgent --priority H \
  --annotate "Stack trace in #general" --annotate "Repo: /Users/neil/Code/..."

# Search and export tasks
ttal task get <uuid>           # export task as rich prompt (inlines referenced docs)
ttal task find <keyword>       # search pending tasks by keyword (OR, case-insensitive)
ttal task find <keyword> --completed  # search completed tasks
```

`ttal task add` validates the project against the ttal project database — use `ttal project list` to see valid aliases. The on-add hook handles `project_path` and `branch` UDAs automatically.

`ttal task get` inlines markdown files from annotations matching `Plan:`, `Design:`, `Doc:`, `Reference:`, or `File:` patterns — useful for feeding full context to agents.


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
