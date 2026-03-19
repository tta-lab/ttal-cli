---
name: breathe
description: "Refresh context window — agent writes handoff and restarts"
claude-code:
  allowed-tools:
    - Bash
---

# Breathe — Session Handoff

When your context window is getting heavy, use this to write a handoff prompt and restart your session with a clean context window. You keep all important state — just shed the conversation weight.

**Announce at start:** "Taking a breath — writing handoff for session restart."

## Usage

```
/breathe
```

## Steps

1. **Write the handoff** as a markdown string (see format below)
2. **Call `ttal breathe`** with the handoff via stdin
3. **Stop** — the daemon will kill this session and restart you with the handoff

## How to Call

```bash
cat <<'HANDOFF_EOF' | ttal breathe
# Session Handoff

## Active Task
[task UUID, description, current status]

## What Was Done
[bullet list — files changed, decisions made]

## Key Decisions
[architecture choices, trade-offs, approach and why]

## Current State
[where you left off — file, step in plan, what you're waiting for]

## Next Steps
[ordered list of what to do next]

## Important Context
[non-obvious things that would be lost — gotchas, workarounds]
HANDOFF_EOF
```

## Quality Checklist

- Task UUID included (not just description)
- Specific file paths (not "the config file")
- Decisions include the *why*
- Next steps are actionable without previous context
- Self-contained — no "earlier in this conversation" references

## What NOT to Include

- Full file contents (new session can read files)
- Conversation history or back-and-forth
- Completed work that doesn't affect next steps
- Tool output or logs (summarize instead)

Target: **50-200 lines** — enough to be useful, short enough to leave room for work.

## What Happens After

1. The daemon receives your handoff
2. Daemon persists your handoff to diary, then reads today's diary entry (handoff + any earlier entries today) as enriched context
3. Writes a synthetic JSONL session with the enriched handoff as the first message
4. Your CC session is killed
5. A new CC session starts with `--resume` on the synthetic session
6. You wake up in a fresh context window with the handoff as context
7. Continue from where you left off

Your handoff is saved in diary — it persists across sessions.

## Auto-Breathe on Route

When a task is routed via `ttal task route`, the agent is asked to breathe
so they start fresh. The router stages routing params to
`~/.ttal/routing/<agent>.json`, then sends a message asking the agent to
`/breathe`. The daemon composes the restart:

- **System prompt (JSONL):** agent's handoff + role prompt from routing file
- **Trigger (positional arg):** task assignment that kicks the agent into action

Managers are exempt — they keep persistent sessions.

To skip: `ttal task route <uuid> --to <agent> --no-breathe`

## Unified Spawn Pattern

All spawns (workers, reviewers, route-breathe) now use the same pattern:

1. Write system prompt into synthetic JSONL session
2. Launch `claude --resume <session-id> -- "<trigger>"`

The system prompt carries context; the trigger kicks off action.
