---
title: "Day 2: The Glue Layer"
description: Multi-channel messaging, webhooks, and workflow orchestration
---

[Day 1](/blog/day-1-the-dream-setup/) ended with a promise: Taskwarrior hooks fire events, and those events can trigger agent workflows.

But here's what I glossed over: events alone are just noise.

A hook fires. Then what? Where does the event go? Who decides what to do with it? What if the action requires multiple steps, or human approval, or coordination across systems?

You need a glue layer. Something that sits between "event happened" and "work got done."

## The Missing Middle

Picture the stack:

```
┌─────────────────────────────────────┐
│  Human Intent (Taskwarrior tasks)   │  ← Day 1
├─────────────────────────────────────┤
│  ??? (something goes here)          │  ← Day 2
├─────────────────────────────────────┤
│  Agent Execution (Zellij sessions)  │  ← Day 3
└─────────────────────────────────────┘
```

Day 1 covered the top layer: tasks as structured data, hooks as events. Day 3 will cover the bottom: how agents actually execute work in isolated sessions.

But the middle layer—the orchestration layer—is what makes autonomous workflows possible. Without it, you're manually wiring every hook to every action. With it, you have a programmable router that handles complexity for you.

## What the Glue Layer Does

The orchestration layer needs to handle:

1. **Event routing** — Hook fires → route to the right handler
2. **Multi-channel I/O** — Accept input from Telegram, Slack, CLI, webhooks
3. **Session management** — Maintain context across interactions
4. **Workflow orchestration** — Multi-step processes with conditions and gates
5. **Notification dispatch** — Send updates to the right channel at the right time

This is where [OpenClaw](https://github.com/openclaw/openclaw) comes in.

## OpenClaw: The Orchestration Platform

OpenClaw is a multi-channel AI assistant platform. For our purposes, it's the programmable router between Taskwarrior events and agent execution.

Core components:

| Component | What it does |
|-----------|--------------|
| **Channel adapters** | Unified interface to WhatsApp, Telegram, Slack, Discord |
| **Webhook handlers** | Receive events from external systems (Taskwarrior hooks) |
| **Lobster workflows** | Multi-step orchestration with approval gates |
| **Session store** | Context persistence across messages and channels |
| **Cron scheduler** | Scheduled task reviews and reminders |

## Two Types of Agents

Here's a distinction that matters: not all agents are the same.

**Manager Team** — Lightweight agents that run inside OpenClaw. They're fast, cheap, and handle coordination:

| Agent | Role |
|-------|------|
| **Researcher** | Find `+research` tasks, investigate, write findings to your knowledge base |
| **Task Agent** | Create tasks, brainstorm, annotate with rich context |
| **Worker Lifecycle Manager** | Spawn, monitor, and clean up execution agents |

The Researcher deserves special mention. Instead of asking questions and waiting for answers, you create a research task:

```bash
task add "What are the tradeoffs between JWT and session tokens?" +research
```

The Researcher picks it up, investigates, and writes a doc to your knowledge base (Obsidian, Notion, whatever). Questions become async. Every answer builds your KB. It's like having a dedicated researcher who works while you sleep.

**Execution Team** — Heavy agents that run in isolated Zellij sessions. They do the actual coding work:

| Agent | Role |
|-------|------|
| **Planner** | Break down tasks, design approach |
| **Coder** | Write and modify code |
| **Reviewer** | Check code quality, catch issues |
| **Tester** | Run tests, verify behavior |

The manager team orchestrates. The execution team executes. OpenClaw runs the managers; Zellij runs the executors.

```
┌─────────────────────────────────────────────────────────┐
│                      OpenClaw                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ Researcher  │  │ Task Agent  │  │ Lifecycle Mgr   │  │
│  └─────────────┘  └─────────────┘  └────────┬────────┘  │
│                                             │           │
└─────────────────────────────────────────────┼───────────┘
                                              │ spawns
                                              ▼
┌─────────────────────────────────────────────────────────┐
│                   Zellij Session                        │
│  ┌──────────┐  ┌────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Planner  │→ │ Coder  │→ │ Reviewer │→ │ Tester   │   │
│  └──────────┘  └────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────┘
```

This separation matters for cost, speed, and reliability. You don't need a heavy coding agent to answer "what's the status of task 42?" The Task Agent handles that in milliseconds. But when it's time to actually fix the bug, the Lifecycle Manager spawns an execution team.

## Memory: Who Remembers What

Here's another key distinction: managers have memory, workers don't.

**Manager agents** maintain their own:
- **Identity** — Personality, communication style, preferences
- **Long-term memory** — Past decisions, learned patterns, project history
- **Workflow knowledge** — How things are done here, what worked before

**Execution agents** are ephemeral. They spin up, do work, spin down. No cross-project memory. No persistent state.

Why? Because context injection is better than global memory for coding tasks. When the Lifecycle Manager spawns a worker, it constructs the perfect prompt:

```
Here's the task: Fix auth timeout bug
Here's the context: Token refresh not triggering (from Task Agent's brainstorm)
Here's the relevant code: auth.ts lines 40-60 (from Researcher's KB)
Here's how we do things: Always add tests, use guard clauses (from project CLAUDE.md)
```

The worker doesn't need to remember your last three projects. It needs exactly the right context for *this* task. Managers curate that context from their memory and inject it at spawn time.

This is why the manager/execution split matters:
- Managers are **stateful** — they learn, remember, improve over time
- Workers are **stateless** — they're disposable experts with perfect context

Your managers become more useful the longer they run. Your workers stay fast and focused because they're not carrying baggage.

## Webhooks: Events In, Actions Out

Taskwarrior hooks are local scripts. They can't reach the internet by themselves. But they can make HTTP requests.

The bridge:

```
task start → on-modify hook → curl webhook → OpenClaw → decide what to do
```

A simplified hook:

```bash
#!/bin/bash
# on-modify hook (conceptual)
read old_task
read new_task

# If task just started, notify OpenClaw
if [[ $(echo "$new_task" | jq -r '.start') != "null" ]]; then
  curl -s -X POST https://your-openclaw/webhook/task-started \
    -H "Content-Type: application/json" \
    -d "$new_task"
fi

echo "$new_task"
```

OpenClaw receives the webhook, parses the task JSON, and the manager team decides what to do:

- **Simple query**: Task Agent answers directly
- **Needs research**: Researcher gathers context first
- **Ready to execute**: Lifecycle Manager spawns execution team

The webhook is the universal adapter. Any system that can make HTTP requests can integrate with OpenClaw.

## Sessions: Context Without Repetition

Stateless systems are simple but frustrating. Every interaction starts from zero.

Sessions solve this:

```
You: "What's the status of task 42?"
AI: "PR #127 is waiting for review. 3 comments addressed."
You: "Merge it"
AI: "Merged. Task 42 marked complete."
```

The session remembers task 42, the PR, the review status. You don't re-explain.

Sessions can be linked to task UUIDs. All context about a task—across channels, across time—lives in one place. The task becomes the coordination point between humans and agents.

## Multi-Channel: Meet Users Where They Are

Why messaging apps? Because you already check them 50+ times a day.

Terminal AI is "pull"—you go to it. Messaging AI is "push"—it comes to you.

Different channels serve different purposes:

- **Telegram**: Power users, automation-friendly, bot ecosystem
- **Slack/Discord**: Team context, shared visibility
- **WhatsApp**: Personal, mobile-first
- **Email**: Async, formal, audit trail

OpenClaw normalizes these channels. A message from Telegram and a message from Slack both become the same internal format. Your workflows don't care where input came from.

## Lobster Workflows: When Simple Triggers Aren't Enough

Simple triggers are "when X, do Y." That covers maybe 30% of real work.

Real work looks like:

1. When task starts, spawn agent
2. Agent creates PR
3. **Wait for human review**
4. If approved, merge and close task
5. If changes requested, agent addresses them, goto step 3
6. If no response in 4 hours, ping human again

This is a workflow—multiple steps, conditional branches, approval gates.

Lobster is OpenClaw's workflow engine. Key concepts:

| Concept | What it does |
|---------|--------------|
| **Steps** | Individual actions (spawn agent, send notification) |
| **Approval gates** | Pause and wait for human decision |
| **Branching** | Different paths based on conditions |
| **Timeouts** | What happens if no response |

Here's a conceptual workflow:

```yaml
name: bug-fix-workflow
trigger: task.started AND task.tags CONTAINS "bug"

steps:
  - spawn_agent:
      task: $task
      workspace: zellij

  - wait_for:
      event: agent.pr_created
      timeout: 2h
      on_timeout: notify_human("Agent stuck, please check")

  - notify:
      channel: telegram
      message: "PR ready: $pr_url"

  - approval_gate:
      prompt: "Merge this PR?"
      options: [approve, request_changes, defer]
      timeout: 4h

  - branch:
      approve:
        - merge_pr
        - complete_task
      request_changes:
        - agent_address_feedback
        - goto: wait_for
      defer:
        - snooze: 1d
```

The approval gate is the key. Agents work autonomously, but humans stay in control. You're not babysitting the terminal, but you're not blindly trusting either.

## Putting It Together: A Bug Fix from the Dog Park

You're walking the dog when you remember a bug that's been bugging you.

**1. You text Telegram:**

```
Hey, I want to fix the auth timeout issue. Can you help me think through it?
```

**2. Task Agent responds** (instantly, from OpenClaw):

```
Sure! Looking at the codebase... The session timeout is set in auth.ts:47.
Current value is 30 minutes. The issue might be:
- Token refresh not triggering
- Session store not updating lastActivity
- Or the timeout value itself is too short

Want me to create a task to investigate? I can add these notes as context.
```

**3. You confirm:**

```
Yes, create a task. Priority high. I think it's the token refresh.
```

**4. Task Agent creates task with rich context:**

```
Created task 47: "Fix auth timeout bug" [project:webapp priority:H]

Annotations added:
- Suspected root cause: token refresh not triggering
- Relevant file: auth.ts:47
- Current timeout: 30 minutes
- Suggested investigation: check refresh logic
```

**5. Later, you're ready to delegate:**

```
Start task 47
```

**6. The manager team coordinates:**

- Task Agent confirms: "Starting task 47"
- Lifecycle Manager spins up a Zellij session
- Execution team receives task context + annotations

**7. Inside Zellij, the execution team works:**

- **Planner** reads annotations, designs fix approach
- **Coder** implements the token refresh fix
- **Reviewer** catches an edge case, Coder addresses it
- **Tester** runs auth tests, all pass

**8. Lifecycle Manager detects completion, notifies you:**

```
Task 47: PR #89 ready for review

Summary:
- Fixed token refresh not triggering on timeout
- Added edge case handling for expired sessions
- All auth tests passing (12 tests)

Files changed:
- src/auth.ts (+15, -3)
- src/auth.test.ts (+45)
```

**9. You review on your phone:**

```
Nice catch on the edge case. LGTM, merge it.
```

**10. Workflow completes:**

```
PR #89 merged to main
Task 47 marked complete
Deployed to staging (auto)
```

You fixed a bug without touching a terminal. The manager team helped you think through the problem and captured rich context. The execution team did the actual coding. You stayed in control through approval gates.

## What's Next

We've covered:
- **Day 1**: Tasks as structured data, hooks as events
- **Day 2**: OpenClaw as the orchestration layer (manager team + workflows)

Next question: What actually happens inside that Zellij session? How does the execution team work together?

[Day 3: Zellij + Coding Agents](/blog/day-3-zellij-coding-agents/) answers that. We'll look at:
- Zellij sessions as isolated workspaces
- How the execution team coordinates (Planner → Coder → Reviewer → Tester)
- Claude Code integration
- The full lifecycle from spawn to cleanup

The glue layer makes autonomous workflows possible. The execution layer makes them real.

---

**Guide series:**
1. [Day 1: The Dream Setup](/blog/day-1-the-dream-setup/)
2. Day 2: The Glue Layer (you are here)
3. [Day 3: Zellij + Coding Agents](/blog/day-3-zellij-coding-agents/)
4. [Day 4: Taskwarrior Deep Dive](/blog/day-4-taskwarrior-deep-dive/)
5. [Day 5+: Topics TBD](/blog/day-5-tbd/)
