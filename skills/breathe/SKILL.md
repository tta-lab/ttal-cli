---
name: breathe
description: "Refresh context window — agent writes handoff and restarts"
category: command
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
6. CC fires the SessionStart hook: `ttal context` renders the universal `context` template, executing `$ cmd` lines (diary read, agent list, pipeline prompt) to build the system message
7. You wake up in a fresh context window with the session context injected by the hook

Your handoff is saved in diary — it persists across sessions.

### Worker and reviewer path (uses SessionStart hook)

Workers and reviewers also use the CC SessionStart hook. On session start:
- The hook renders the `context` template
- `TTAL_JOB_ID` is derived from the worktree CWD (workers) or session env (reviewers)
- `$ ttal pipeline prompt` outputs the role-specific prompt (coder instructions, review prompt, etc.)

## Auto-Breathe on Route

When a task is routed via `ttal go`, the agent is asked to breathe so they start fresh.
The stage tag is already written to taskwarrior before breathe is triggered.
On next startup, the SessionStart hook renders the context template and `$ ttal pipeline prompt`
reads the stage tag to output the role-specific prompt. No route file needed — taskwarrior
state is the single source of truth.

To skip: `ttal go <uuid> --no-breathe`

## Unified Context Injection

All session types (manager, worker, reviewer) use the same `context` template rendered by
the CC SessionStart hook:

1. Daemon kills old session (managers) or worker spawns directly with `--agent`
2. CC starts, fires SessionStart hook
3. `ttal context` renders the `context` template — `$ cmd` lines executed with agent env vars
4. System message injected into the session

The SessionStart hook is installed via `ttal sync` (not `ttal doctor --fix`).
