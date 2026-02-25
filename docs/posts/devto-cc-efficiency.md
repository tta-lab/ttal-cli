# I shipped 706 commits in 5 days with Taskwarrior + Claude Code

> How Taskwarrior + Zellij + Claude Code enabled CC founder-level productivity without burning out

Last week I merged 38 PRs across 5 repos. 706 commits. One person, max 5 Claude Code sessions at a time.

I'm sharing this because I think most CC users are hitting the same ceiling I was.

## The ceiling

If you use Claude Code, you've probably tried scaling up to multiple sessions. Open a few terminals, give each one a task, and... immediately start context-switching between them. Which session just finished? What does this one need from that one? Are two sessions editing the same file?

The CC founder reportedly runs 10+ parallel sessions. The difference isn't superhuman multitasking. It's a system that eliminates the coordination overhead.

## The stack

I call it **TTAL** — The Taskwarrior Agents Lab. Three tools:

| Tool | Role |
|------|------|
| [Taskwarrior](https://taskwarrior.org/) | Task queue + event system |
| [Zellij](https://zellij.dev/) | Terminal session manager |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | The agent that does the work |

Taskwarrior hooks spawn Zellij panes. Each pane runs a CC session with task context injected. When a session finishes, the next highest-urgency task auto-starts. You don't manage sessions. You manage tasks.

```
Mon: 199 commits — voice/ASR pipeline + agent heartbeat system
Tue: 182 commits — backend features + TUI contributions
Wed: 122 commits — infrastructure + documentation
Thu:  49 commits — rate-limited, did reviews instead
Fri: 154 commits — config consolidation + new features
```

Thursday is the tell — API rate limit hit, throughput dropped 75%. The system was the bottleneck, not me.

## On-demand human-in-the-loop

This is the design principle that makes it click: **agents never block waiting for me**.

Most CC workflows are synchronous — you give a task, watch it work, review, give the next task. You are the bottleneck at every step.

In TTAL, agents pick up tasks, do the work, commit, and move on. I review PRs *when I'm ready* — not when the agent needs me. That's why 5 async sessions outperform 10 synchronous ones.

The bottleneck was never the AI. It was the glue.
