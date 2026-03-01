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
ttal task get <uuid>
```

This exports a task as a rich prompt, suitable for piping to agents. It includes:
- Task description and annotations
- Inlined content from annotation paths matching `Plan:`, `Design:`, `Doc:`, `Reference:`, or `File:` patterns

Accepts 8-character UUID prefixes or full UUIDs.

## Task routing

Route tasks to specialized agents based on what needs to happen:

```bash
# Send to design agent — writes an implementation plan
ttal task design <uuid>

# Send to research agent — investigates and writes findings
ttal task research <uuid>

# Send to test agent
ttal task test <uuid>

# Spawn a worker to implement
ttal task execute <uuid>
```

### Configuring route targets

Set which agent handles each role in `config.toml`:

```toml
[teams.default]
design_agent = "inke"       # ttal task design → inke
research_agent = "athena"   # ttal task research → athena
test_agent = "sage"         # ttal task test → sage
```

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
```

## Tag-based dispatch

Tags trigger automatic routing to specific agents:

```bash
# Tasks tagged +research route to the research agent
task add "Investigate auth library options" +research

# Custom tag routes are configurable
task add "Create new deployment skill" +newskill
```

Built-in tag routes include `+newskill` (skill creation workflow) and `+newagent` (agent creator).
