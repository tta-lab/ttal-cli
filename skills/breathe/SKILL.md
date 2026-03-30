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

### Manager path (uses SessionStart hook)

1. The daemon receives your handoff
2. Daemon persists your handoff to diary
3. Status file session ID is cleared
4. Your CC session is killed
5. A new CC session starts fresh (no `--resume`)
6. CC fires the SessionStart hook: `ttal context` reads env, evaluates breathe_context commands, and consumes any pending route file to build the system message
7. You wake up in a fresh context window with the session context injected by the hook
8. Continue from where you left off

Your handoff is saved in diary — it persists across sessions.

### Worker path (still uses synthetic JSONL + --resume)

Workers use synthetic JSONL sessions with `--resume` — the SessionStart hook does not apply to short-lived worker sessions.

## Auto-Breathe on Route

When a task is routed via `ttal go`, the agent is asked to breathe
so they start fresh. The router stages routing params to
`~/.ttal/routing/<agent>.json`, then sends a message asking the agent to
`/breathe`. On next startup:

- **Manager path:** The SessionStart hook (`ttal context`) consumes the route file and injects the role prompt and task assignment as the system message
- **Worker path:** The daemon reads the route file during `handleBreathe` and composes the restart

Managers are exempt from forced breathe — they keep persistent sessions.

To skip: `ttal go <uuid> --no-breathe`

## Unified Spawn Pattern (Manager)

The manager breathe path:

1. Daemon kills old session, creates new session without `--resume`
2. CC starts, fires SessionStart hook
3. `ttal context` evaluates breathe_context commands and pending route file
4. System message is injected into the session

The SessionStart hook is installed via `ttal sync` (not `ttal doctor --fix`).
