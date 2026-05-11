---
name: breathe
description: "Refresh context window — agent writes handoff and restarts"
category: command
claude-code:
  allowed-tools:
    - Bash
---
# Breathe — Session Handoff

When your context window is getting heavy, use this to write a handoff to your diary and restart your session with a clean context window. You keep all important state — just shed the conversation weight.

**Announce at start:** "Taking a breath — writing handoff for session restart."

## Usage

```
/breathe
```

## Steps

1. **Write the handoff to your diary** via heredoc into `diary {agent} append`
2. **Call `ttal breathe`** (no arguments)
3. **Stop** — the daemon will kill this session and restart you with a fresh context window

## How to Call

```
cat <<'HANDOFF_EOF' | diary $TTAL_AGENT_NAME append
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

ttal breathe
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

### Manager path

1. The daemon receives your `ttal breathe` request
2. Status file session ID is cleared
3. Your session is killed
4. A new session starts fresh (same runtime, no resume)
5. Your spawn trigger says to run `ttal context` for your briefing
6. `ttal context` picks the manager template, renders it (diary read, agent list, project list, pairing, role prompt, task), and prints the bundle
7. You wake up in a fresh context window

Your handoff is in your diary — you wrote it there in step 1, and `ttal context` reads diary as part of the wake bundle.

### Worker and reviewer path

Workers and reviewers wake via the unified spawn trigger. On session start:
- Run `ttal context` — it picks the worker template and prints the bundle
- `TTAL_JOB_ID` is set by the spawn parent (or derived from worktree CWD)
- Pairing, role prompt with inlined skills, and task body are in the output

## Auto-Breathe on Route

When a task is routed via `ttal go`, the agent is asked to breathe so they start fresh.
The stage tag is already written to taskwarrior before breathe is triggered.
On next startup, your spawn trigger says to run `ttal context`.
`ttal context` picks the right template and prints the bundle — pairing, role prompt with
inlined skills, and task. No route file needed — taskwarrior state is the single source of truth.

To skip: `ttal go {uuid} --no-breathe`

## Unified Context Injection

All session types (manager, worker, reviewer) use `ttal context`:

1. Spawn trigger: "Run `ttal context` for your briefing, then act on the role prompt."
2. Agent runs `ttal context`
3. Picks manager or worker template by checking agentfs for an AGENTS.md under team_path
4. Renders the template — `$ cmd` lines execute with agent env vars (TTAL_AGENT_NAME, TTAL_JOB_ID)
5. Prints the bundle to stdout — no hook, no size budget