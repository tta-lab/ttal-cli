---
name: plan-review
description: "Spawn a plan reviewer for the current task"
argument-hint: "<task-uuid>"
---

# Plan Review

Spawn a plan-reviewer tmux window for the given task:

```bash
ttal plan review <uuid>
```

If a plan-review window is already running, this sends a re-review request to it instead.
