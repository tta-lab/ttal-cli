# Day 1: The Dream Setup

> Why Taskwarrior is the secret sauce that connects human intent to autonomous agents

I was walking my dog when I realized I've been driving stick shift my whole career.

Every project, every bug fix, every feature request—I'm manually shifting gears. Check the task. Open the right files. Remember where I left off. Context-switch to the next thing. Repeat until exhausted.

What if I could set the destination and let the car drive itself?

## The Manual Transmission Problem

Here's what a typical workday looks like:

1. Check task list
2. Pick a task
3. Open the right project/files
4. Remember what I was doing yesterday
5. Do some work
6. Get interrupted
7. Context-switch to urgent thing
8. Forget everything about the first task
9. Repeat

Each context switch costs 20+ minutes of mental reload. By the end of the day, you've spent more time switching than doing.

## The Dream: Autonomous Workflows

What if starting a task could:
- Automatically open the right workspace
- Load relevant context into an AI agent
- Spawn workers to handle subtasks
- Clean everything up when you're done

This isn't science fiction. It's Taskwarrior + hooks + the right glue code.

## Why Taskwarrior?

Taskwarrior seems boring. It's old. It's CLI-based. It does have sync (Taskchampion) and mobile apps, but they're not the point—you won't find "login with Google" or AI chat features here.

But that's exactly why it works.

### 1. Tasks as Structured Data

Every task in Taskwarrior is just JSON:

```json
{
  "uuid": "abc-123",
  "description": "Fix login bug",
  "project": "webapp",
  "tags": ["bug", "urgent"],
  "priority": "H",
  "annotations": [
    {"description": "Root cause: session timeout", "entry": "2024-01-15"}
  ]
}
```

This isn't a database row locked in some app. It's portable, queryable, scriptable data.

### 2. Hooks: The Bridge Layer

Taskwarrior has hooks. Simple scripts that fire when things happen:

- `on-add` - task created
- `on-modify` - task changed (started, completed, edited)
- `on-exit` - after any command

When you run `task 1 start`, Taskwarrior:
1. Reads the task
2. Sets `status: active` and `start: now`
3. Runs your `on-modify` hook with the before/after JSON
4. Your script can do *anything*

```
task start → on-modify hook → spawn worker → do work
task done  → on-modify hook → cleanup → archive
```

### 3. Powerful Query Language

Need all high-priority bugs in the webapp project?

```bash
task project:webapp +bug priority:H list
```

Need tasks I touched this week?

```bash
task modified:week list
```

Need custom output format?

```bash
task project:webapp export
```

It's SQL for your life, without the database overhead.

### 4. Smart Urgency Ranking

Here's where it gets interesting for agents. Taskwarrior auto-calculates an **urgency score** for every task:

| Factor | Coefficient |
|--------|-------------|
| +next tag | 15.0 |
| Due date approaching | 12.0 |
| Blocking other tasks | 8.0 |
| Priority H / M / L | 6.0 / 3.9 / 1.8 |
| Age of task | 2.0 |
| Has annotations | 1.0 |
| Blocked by others | -5.0 |

Query by urgency and the most important task floats to the top:

```bash
task limit:1 export   # Returns highest urgency task as JSON
```

Agents don't need to figure out what to work on—Taskwarrior tells them. And the coefficients are customizable in `.taskrc`.

### 5. Universal Applicability

This isn't just for programmers. Taskwarrior's UDAs (User Defined Attributes) let you add custom fields for any domain:

```bash
# Define a phase field with your workflow stages
uda.phase.type=string
uda.phase.values=draft,edit,review,publish

# Now use it
task add "Write blog post" phase:draft
task 1 modify phase:review
```

The same pattern works for any workflow:

- **Content creators**: `phase:draft` → `phase:edit` → `phase:review` → `phase:publish`
- **Sales teams**: `phase:lead` → `phase:qualified` → `phase:proposal` → `phase:closed`
- **Personal goals**: `phase:idea` → `phase:planned` → `phase:active` → `phase:done`

Your hooks can read `task.phase` and trigger different automations at each stage. A content creator's `phase:review` might notify an editor. A sales team's `phase:proposal` might generate a quote document.

The hook system works identically across all these domains. `task start` triggers your automation whether you're fixing bugs or writing blog posts.

## The Communication Layer

Here's the insight that changes everything: **annotations are a shared communication channel**.

Humans, orchestrator agents, and workers all write to the same task annotations. Every important message about a task lives in one place:

```bash
task 1 annotate "Design: Use webhook pattern for loose coupling"
task 1 annotate "Worker: PR #42 created, awaiting review"
task 1 annotate "Decision: Approved by human, proceed with merge"
task 1 annotate "Worker: Merged and deployed to staging"
```

Query any task and you see the full conversation:

```bash
task 1 info
# Shows: description, project, tags, all annotations with timestamps
```

This changes how you work. When context is complete in the annotations:
- You stop repeating the same instructions to agents
- You focus on making tasks more complete, not re-explaining them
- You plan the next move instead of babysitting the current one

The task becomes the **single source of truth**. Agents read it, humans read it, everyone stays aligned without chat threads scattered across Slack, email, and docs.

## The Glue: OpenClaw

This is where [OpenClaw](https://github.com/openclaw/openclaw) comes in. It's a multi-channel AI assistant platform that connects messaging apps (WhatsApp, Telegram, Slack, Discord) to AI agents—but more importantly for us, it has the automation primitives we need:

- **Webhooks** that Taskwarrior hooks can call
- **Lobster workflows** for multi-step orchestration with approval gates
- **Cron jobs** for scheduled task reviews
- **Session management** for maintaining context across conversations

The pattern becomes:

1. [Taskwarrior](https://taskwarrior.org/) hook fires → calls OpenClaw webhook
2. OpenClaw spawns an agent session with task context
3. Agent works in [Zellij](https://zellij.dev/) with [Claude Code](https://claude.com/product/claude-code)
4. On completion, webhook fires again → cleanup

The architecture isn't locked to any specific tool. Swap Claude Code for opencode, aider, or crush—[Zellij](https://zellij.dev/) sessions don't care what CLI runs inside them. Desktop apps—browser automation, Excel, PowerPoint—can plug into the same webhook system alongside terminal-based agents.

The details will come in future posts. The key insight is: [Taskwarrior](https://taskwarrior.org/)'s hook system provides the events, OpenClaw provides the orchestration layer.

## What's Next

In [Day 2: The Glue Layer](day-2-orchestration-layer.md), we'll look at the orchestration layer—webhooks, Lobster workflows, and how external events trigger agent sessions.

For now, the takeaway: if you want autonomous workflows, you need structured task data with an event system. Taskwarrior has been quietly doing this for years.

---

**Guide series:**
1. Day 1: The Dream Setup (you are here)
2. [Day 2: The Glue Layer](day-2-orchestration-layer.md)
