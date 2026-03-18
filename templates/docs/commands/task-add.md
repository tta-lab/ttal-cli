---
name: task-add
description: "Create taskwarrior tasks from a plan, list, or description"
argument-hint: "<plan-path or task descriptions>"
claude-code:
  allowed-tools:
    - Bash
    - Read
---

# Task Create

Create one or more taskwarrior tasks from the given input.

## Input Modes

1. **Plan file path** — e.g. `/task-create ~/clawd/docs/plans/2026-02-26-foo.md`
   Read the plan, extract action items, create tasks with proper project tags and annotations.

2. **Inline task descriptions** — e.g. `/task-create "Fix the login bug" "Add tests for auth"`
   Create each as a separate task.

3. **No arguments** — Read the current conversation context for tasks to create.

## Workflow

1. Parse input to identify tasks
2. For each task, create via `ttal task add`:
   ```bash
   ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"
   ```
3. Report created tasks with UUIDs

## Rules

- Use `ttal task add` for all task creation — it validates projects and handles conventions
- Never run raw `task add` — use `ttal task add` instead
- If creating from a plan file, use `--annotate "Plan: <path>"` to link back
- Tags and annotations are repeatable (`--tag bugfix --tag urgent --annotate "note1" --annotate "note2"`)
- Group related tasks and set dependencies where appropriate (`task <uuid> modify depends:<other-uuid>`)
