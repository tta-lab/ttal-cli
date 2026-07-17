---
title: Pipeline Jobs
description: Existing Taskwarrior-backed pipeline lifecycle
---

ttal still uses Taskwarrior internally for its existing pipeline lifecycle, but it no longer exposes task creation, search, prompt export, or heatmap commands.

Use FlickNote for durable goals, designs, plans, and research:

```
flicknote add "Design or plan content" --project orientation
flicknote find auth login
flicknote detail <id>
```

## Pipeline routing

Advance an existing pipeline job:

```
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

The configured pipeline stage and agent roles determine the route.


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
