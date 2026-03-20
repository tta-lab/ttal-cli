---
title: Research → Design → Execute
description: The standard ttal task pipeline
---

The typical ttal workflow follows three phases: research the problem, design the solution, then execute the implementation. Each phase produces artifacts that flow automatically to the next.

## The pipeline

### 1. Research

```bash
ttal task go <uuid>
```

The research agent investigates the problem, explores options, and writes a findings document. It then annotates the task:

```
Research: ~/clawd/docs/research/2026-03-01-auth-options.md
```

### 2. Design

```bash
ttal task go <uuid>
```

The design agent reads the task (including the research findings via the annotation), writes an implementation plan, and annotates the task:

```
Plan: ~/clawd/docs/plans/2026-03-01-auth-implementation.md
```

### 3. Execute

```bash
ttal task go <uuid>
```

This spawns a worker in a tmux session with a git worktree. The worker receives the full task context — including the research findings and implementation plan, automatically inlined from annotations.

## Automatic context flow

The key feature: **`ttal task get` inlines all annotation paths**.

When a task has annotations like:
```
Plan: ~/clawd/docs/plans/auth-implementation.md
Research: ~/clawd/docs/research/auth-options.md
```

The worker's prompt automatically includes the full content of those files. No manual copy-paste — context flows from research → design → worker automatically.

## Example: Adding authentication

### Step 1: Create the task

```bash
ttal task add --project myapp "Add JWT authentication to the API"
```

### Step 2: Research

```bash
ttal task go <uuid>
```

Athena (the research agent) investigates JWT libraries, compares options, and writes findings.

### Step 3: Design

```bash
ttal task go <uuid>
```

Inke (the design agent) reads Athena's research, writes an implementation plan with specific files to modify, and annotates the task.

### Step 4: Execute

```bash
ttal task go <uuid>
```

A worker spawns with full context: the task description, Athena's research, and Inke's plan — all inlined in the prompt. The worker follows the plan, implements the feature, creates a PR.

### Step 5: Review and merge

The PR goes through automated review (6 specialized review agents), the worker triages feedback, and you merge from Telegram.

## When to skip phases

Not every task needs all three phases:

- **Simple bug fix** — skip research and design, go straight to `ttal task go`
- **Well-understood feature** — skip research, go straight to design stage with `ttal task go <uuid>`, then execute
- **Exploratory work** — research only, then decide next steps based on findings
