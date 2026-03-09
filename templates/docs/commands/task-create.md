---
name: task-create
description: "Create taskwarrior tasks from a plan, list, or description"
argument-hint: "<plan-path or task descriptions>"
claude-code:
  context: fork
  allowed-tools:
    - Bash
    - Read
    - Task
opencode: {}
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
2. For each task, dispatch to the **task-creator** subagent:
   - Use Task tool with `subagent_type: "task-creator"`
   - Include task description, project context, and any annotations
3. Report created tasks with UUIDs

## Rules

- **Never run `task add` directly** — always use the task-creator subagent
- task-creator handles project validation, tag conventions, and annotations
- If creating from a plan file, annotate each task with the plan path
- Group related tasks and set dependencies where appropriate
