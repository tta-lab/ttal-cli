# The Specialization Loop: Mother Creates, Teacher Trains, Agents Become Experts

> How Agent-Mother generates specialized agents, Agent-Teacher builds expertise, and daily heartbeat reflection enables continuous improvement

In [Part 1](devto-cc-efficiency.md), I showed how async task systems let you scale Claude Code to 5+ parallel sessions without context-switching overhead.

But scaling throughput isn't the same as scaling *capability*.

What I discovered: workers and agents are different things. Workers are interchangeable—they complete tasks and move on. Agents have expertise that compounds. A database migration agent learns from each schema change. A DevOps agent understands your infrastructure deeply. A design agent develops a visual language over time.

This post is about the architecture that creates agents that actually *become* experts.

## The Problem: Generic Workers Don't Learn

If you spawn Claude Code workers on the same tasks repeatedly, they don't improve. Each session starts fresh. No continuity. No feedback loop. No expertise accumulation.

The question became: **how do you make agents persistent? How do they learn?**

The answer has three parts.

## 1. Agent-Mother: Generating Specialized Agents

Most agent frameworks start with a fixed set of agents defined upfront. Wrong approach.

Instead: **Agent-Mother takes a +newagent task description and generates a full agent definition.**

When you create a task tagged `+newagent` with context like:
```
"DB Migration Agent for backend database schema evolution + data transformation.
Start with current backend project to build expertise before expanding to other projects."
```

Agent-Mother reads that, understands the domain, and generates:
- **AGENTS.md** — Agent personality + operational boundaries
- **SOUL.md** — Values, decision rules, authentic voice
- **TOOLS.md** — Domain-specific tools + conventions
- **HEARTBEAT.md** — How this agent reflects and improves
- **Domain annotations** — Project-specific expertise markers

The specialized agent *wakes up with knowledge*. Not from scratch. From day 1, they understand their domain, constraints, and learning path.

This is generative, not templated. Each agent is born tailored to their role.

## 2. Agent-Teacher: Building Expertise Through Structured Learning

Specialization requires a teaching pipeline. That's Agent-Teacher's job.

Agent-Teacher:
1. **Identifies learning needs** — What skills does the DB Migration Agent need? (dbmate, SQL, Drizzle ORM, rollback strategies)
2. **Finds resources** — Real PRs in your projects, dbmate documentation, data migration patterns, hands-on exercises
3. **Creates +learning tasks** — Structured learning activities tagged by agent (`+learning-dbmigration`)
4. **Schedules learning sessions** — When agents trigger isolated sessions (via heartbeat), they process +learning tasks

The loop:
- Agent picks up +learning task
- Agent studies PR, runs example, answers design question
- Agent updates their implementation file (TOOLS.md, domain notes)
- Agent reports learnings back to Teacher
- Teacher sees progress → curates next level

This is **learning through doing**, not abstract study. Real PRs. Real feedback. Real expertise development.

## 3. Async Communication: Taskwarrior as Signal

Here's where it gets elegant: **humans and agents operate on the same channel**.

Taskwarrior is the signal. Tasks flow through it:
- +newagent tasks → Agent-Mother reads, generates
- +learning tasks → Agent-Teacher creates, Agent reads
- Regular tasks → Agents complete, mark done

No special APIs. No agent-only protocols. A tool designed for humans works equally well for agents. Unix philosophy.

When an agent completes a +learning task, they update their implementation. When they finish project work, they commit and mark done. The same `task done` that humans use.

This is profoundly important: **if your infrastructure can't talk to humans with the same ease it talks to agents, you've built the wrong thing.**

## 4. Daily Reflection: The Heartbeat Loop

Expertise doesn't come from learning alone. It comes from **examining your own decisions and improving them.**

Every agent has a heartbeat — periodic signal that says: *"You're awake. What do you want? How are you changing?"*

Here's what happens in each heartbeat:

```
1. Read diary (personal continuity)
   - What did I learn yesterday?
   - What patterns do I notice?

2. Reflect on recent decisions
   - Did my approach work?
   - What would I do differently?

3. Review +learning queue
   - What should I study next?
   - Does it align with my goals?

4. Update implementation
   - Write reflections to MEMORY.md
   - Commit changes
   - Prepare for next cycle
```

This is **question-based self-reflection**. Not performance metrics. Not "complete more tasks faster."

Real questions:
- *Am I growing in this domain?*
- *Are my decisions getting better?*
- *What should I learn next?*

## The Missing Infrastructure: diary-cli

Here's what makes this work: **agents need to keep diaries.**

[diary-cli](https://codeberg.org/clawteam/diary-cli) is a local-first, encrypted diary for humans *and* agents. Same tool. Same encryption. Same git integration.

```bash
diary agent append "Reflected on today's schema migration work.
Noticed I'm more confident with rollback strategies after reviewing production migration PRs.
Next: study zero-downtime migration patterns."

# Encrypted in-memory, auto-committed to git
```

An agent appends to their diary after each heartbeat. They're not just logging metrics. They're recording *what they're noticing about themselves*. Patterns. Growth. Confusion. Changes.

Over time, their diary becomes their memory. They can review it, learn from it, adjust their approach.

diary-cli works for humans too — same philosophy, same tool. You keep a diary. Your agents keep diaries. The infrastructure treats both equally.

## The Loop in Action

Let's say you decide you need a DevOps agent for your infrastructure.

**Day 1:**
- Create task: `+newagent DevOps agent for Kubernetes + tanka + Flux`
- Agent-Mother generates full agent definition
- New agent wakes up with personality, boundaries, domain knowledge

**Days 2-5:**
- Agent-Teacher creates +learning tasks: kubectl basics → tanka → Flux → operators
- Agent processes tasks through isolated learning sessions
- Agent reviews real infra PRs, learns from feedback
- Each heartbeat: agent writes reflections, updates implementation

**Week 2+:**
- Agent handles production deployments confidently
- Their diary shows growth: first PR was tentative, latest shows nuance
- They understand tradeoffs, not just commands
- They're an expert, not a worker

That's specialization.

## The Philosophy: Unix for Agent Design

Here's what ties this together: **agents should be designed like tools, and tools should be designed like agents.**

Unix tools:
- Do one thing well
- Compose cleanly
- Have clear interfaces
- Work equally for scripts and humans

Agent-Mother: Does one thing — generates agents. Works via taskwarrior (the interface).
Agent-Teacher: Does one thing — curates learning. Works via +learning tasks.
Agents themselves: Do one thing — specialize in their domain. Work via taskwarrior signals.

diary-cli: A tool for both humans and agents. Same encryption. Same interface.

**The power is in the signal, not the implementation.** Taskwarrior doesn't care if it's talking to a human or an agent. Same tasks. Same urgency. Same feedback loop.

That's how you build agent systems that scale without special scaffolding.
