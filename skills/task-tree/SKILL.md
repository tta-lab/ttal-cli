---
name: task-tree
description: Using the taskwarrior fork's subtask trees as executable plans
---

# Task Tree

## Overview

The subtask tree IS the plan. Each subtask = a step. Body text under a heading = annotation with details. No translation step between plan and work — workers read the tree directly.

Two tools, two purposes:
- **flicknote** — orientation docs (what/why): goals, anti-goals, trade-offs, context, architecture decisions
- **task tree** — execution plans (how/steps): subtask hierarchy, each subtask = a step

## Creating a Plan

`task plan` is for **decomposition** — building a subtask tree from scratch by breaking a goal into steps. Think of it as the "outline" phase.

**Not** for surgical edits to individual subtasks. If you already have a tree and just need to update one subtask's details, use `task annotate` or `task modify` (see Common Mistakes below).

Pipe markdown to `task <parent-uuid> plan`:

```bash
cat <<'PLAN' | task <parent-uuid> plan
## Add validation layer
Add input validation to the API handler. Check required fields, validate types, return 400 on failure.

## Write tests
Unit tests for validation: missing fields, wrong types, valid input.

## Update error responses
Standardize error response format to match API conventions.
PLAN
```

View what you created:

```bash
task <parent-uuid> tree
```

Each `##` heading becomes a direct subtask. Body text becomes the subtask's annotation. Use `###` for sub-subtasks when a step needs its own breakdown.

## Markdown Format

| Markdown | What it creates |
|----------|----------------|
| `# Title` or `## Title` | Direct subtask of parent |
| `### Title` | Sub-subtask (child of the subtask above) |
| `#### Title` | One level deeper |
| Text after a heading | Annotation on that subtask |

## Iterating on a Plan

```bash
# Replace the entire subtask tree with a new version
# ⚠️ Destructive: drops ALL existing subtasks before creating new ones
cat updated-plan.md | task <parent-uuid> plan replace

# Append more subtasks (keeps existing ones)
cat more-steps.md | task <parent-uuid> plan

# Add a single subtask manually
task add "Deploy and verify" parent_id:<parent-uuid>

# Reorder subtasks
task <subtask-uuid> modify before:<other-subtask-uuid>
task <subtask-uuid> modify after:<other-subtask-uuid>

# Move a subtask to a different parent
task <subtask-uuid> modify parent_id:<new-parent-uuid>

# Promote a subtask to root level
task <subtask-uuid> modify parent_id:
```

## Viewing and Checking

```bash
# View the full subtask tree
task <uuid> tree

# View tree filtered by project
task project:ttal tree

# View task details including parent and children
task <uuid> information
```

## Completing Work

```bash
# Complete a parent -- cascades to ALL descendants automatically
task <parent-uuid> done

# Complete a single subtask -- only that subtask, siblings/parent unaffected
task <subtask-uuid> done
```

## Handoff to Workers

Workers read their subtask tree to know what to do. The plan review process uses `task <uuid> tree` to review the plan structure. No separate annotation linking is needed — the subtasks are already under the parent task.

## Common Mistakes

### Treating `task plan` as a surgical editor

`task plan` (without `replace`) **appends** subtasks. `task plan replace` **replaces the entire tree**. Neither is safe for editing one subtask's details — you'd lose the rest of the tree.

For editing individual subtask details, use:
```bash
# Update a subtask's description
task <subtask-uuid> modify "Updated description"

# Add or update annotation text on a subtask
task <subtask-uuid> annotate "Additional detail: ..."

# Change a subtask's status
task <subtask-uuid> start
task <subtask-uuid> done
```

### Creating duplicate subtask trees

Agents sometimes write detailed plans in task annotations, then also pipe similar markdown to `task plan`, resulting in two copies of the same work. The tree is the plan — don't duplicate it in annotations.

- Use **subtask tree** for the execution steps (workers read this)
- Use **flicknote** for orientation context (goals, trade-offs, architecture)

### Using `task plan replace` casually

`task plan replace` drops ALL existing subtasks before creating new ones. It's destructive. Prefer appending or creating from scratch in a fresh task. Only use `replace` when you intentionally want a full rebuild.
