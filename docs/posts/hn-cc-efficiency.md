# Show HN: TTAL – I shipped 706 commits in 5 days with Taskwarrior + Claude Code

706 commits, 38 PRs merged, 5 repos, 5 days. One person with max 5 Claude Code sessions.

The stack: Taskwarrior as the task queue, Zellij as the session manager, Claude Code as the worker. Each CC session runs in a Zellij pane with a task assigned. When a session finishes, the next highest-urgency task auto-starts via Taskwarrior hooks. You manage tasks, not sessions.

Thursday I got API rate-limited and throughput dropped 75%. That's the proof - the system was the bottleneck, not me.

The key design: on-demand human-in-the-loop. Agents never block waiting for me. They pick up tasks, do the work, commit, and move on. I review PRs and make decisions when I'm ready - not when the agent needs me. That's what eliminates the human-as-bottleneck problem. 5 sessions is plenty when none of them are waiting on you.

The architecture isn't locked to Claude Code - Zellij sessions don't care what CLI agent runs inside them.
