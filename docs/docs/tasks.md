---
title: Tasks
description: Task management with Taskwarrior integration
---

ttal integrates with [Taskwarrior](https://taskwarrior.org/) for task-driven agent workflows. Tasks are the unit of work — agents pick up tasks, work on them, and report back.

## Finding tasks

```bash
# Search by keyword (OR logic, case-insensitive)
ttal task find auth login

# Search completed tasks
ttal task find auth --completed
```

## Getting task details

```bash
ttal task get
```

This exports a task as a rich prompt, suitable for piping to agents. It includes:
- Task description and annotations
- Inlined content from annotation paths matching `Plan:`, `Design:`, `Doc:`, `Reference:`, or `File:` patterns

The UUID is auto-resolved from `$TTAL_JOB_ID` (worker sessions) or `$TTAL_AGENT_NAME` (agent sessions) — no parameter needed.

## Task routing

Route tasks to specialized agents based on what needs to happen:

```bash
# Advance to next pipeline stage (routes to agent or spawns worker)
ttal go <uuid>
```

### Configuring route targets

Agents declare their role in CLAUDE.md frontmatter. The role maps to a `[prompts]` key:

```yaml
# In agent's CLAUDE.md frontmatter:
---
role: designer
---
```

```yaml
---
role: researcher
---
```

Use `ttal go <uuid>` to route to any agent by name (role determines which prompt is used).

## Today's focus

Manage your daily task focus using taskwarrior's `scheduled` date:

```bash
# List today's focus tasks (sorted by urgency)
ttal today list

# Show tasks completed today
ttal today completed

# Add tasks to today's focus
ttal today add <uuid> [uuid...]

# Remove from today
ttal today remove <uuid> [uuid...]
```

## Enrichment hooks

When tasks are created, ttal's `on-add` hook automatically enriches them with metadata:

- **`project_path`** — the filesystem path to the relevant project
- **`branch`** — a suggested branch name derived from the task description

This enrichment runs inline, populating taskwarrior UDAs (User Defined Attributes) so that when a worker spawns, it knows exactly where to work and what branch to create.

### Required UDAs

Add these to your `~/.taskrc`:

```
uda.branch.type=string
uda.branch.label=Branch

uda.project_path.type=string
uda.project_path.label=Project Path

uda.pr_id.type=string
uda.pr_id.label=PR ID

uda.owner.type=string
uda.owner.label=Owner
```

