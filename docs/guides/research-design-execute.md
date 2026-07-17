---
title: Research → Design → Execute
description: A FlickNote-based workflow with user-controlled execution
---

The workflow has three distinct phases. FlickNote carries durable context between them; moving to implementation is a user decision.

## 1. Research

Investigate the problem and store findings in the `research` project:

```
flicknote add "research findings" --project research
```

Return the note ID. Research does not create tasks or start design automatically.

## 2. Design and planning

Use the research note as input. Brainstorm the direction, then store the agreed design or implementation plan in `orientation`:

```
flicknote add "agreed design or plan" --project orientation
```

The note is the source of truth for goals, anti-goals, approach, implementation stages, tests, exit criteria, and risks.

## 3. Execute

When the goal is ready, the user explicitly starts `goal-impl` with the FlickNote plan ID. No planning skill starts implementation automatically.

The implementation agent reads the note, works on a branch, verifies the result, and submits a PR. `goal-review` checks the implementation against the same note.

## Existing TTAL pipelines

Existing Taskwarrior-backed jobs can still advance with:

```
ttal go <uuid>
```

TTAL no longer exposes task creation, search, prompt export, or task heatmap commands.

## When to skip phases

- Mechanical change: implement directly with proportionate verification.
- Clear but non-trivial change: write an `orientation` plan, then let the user start implementation.
- Unclear problem: research or brainstorm first.
- Large plan: split it into independently deliverable phases in the FlickNote note.
