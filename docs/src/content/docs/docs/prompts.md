---
title: Prompts
description: Customize what gets sent to agents and workers
sidebar:
  order: 9
---

ttal lets you customize the prompts sent to agents when routing tasks. This controls what instructions your design, research, test, and execute agents receive.

## Configuration

Add a `[prompts]` section to your `config.toml`:

```toml
[prompts]
design = "Design an implementation plan for this task: {{task-id}}"
research = "Research this topic and write findings: {{task-id}}"
test = "Write and run tests for this task: {{task-id}}"
execute = "/sp-executing-plans"
```

## Template variables

- **`{{task-id}}`** — replaced with the task's short UUID at runtime

## How each prompt is used

### `execute`

The execute prompt is prepended to the worker's spawn prompt. When you run `ttal task execute <uuid>`, the worker receives this prompt followed by the task context (description, annotations, inlined docs).

Default: invokes the executing-plans skill to implement the task step by step.

```toml
[prompts]
execute = "/sp-executing-plans"
```

### `design`

Controls what gets sent when you run `ttal task design <uuid>`. The design agent receives this prompt with the task UUID, reads the task details, and writes an implementation plan.

Default: asks the agent to write a plan document and annotate the task with its path.

### `research`

Controls what gets sent when you run `ttal task research <uuid>`. The research agent receives this prompt, investigates the topic, and writes findings.

Default: asks the agent to research the topic and annotate the task with the findings path.

### `test`

Controls what gets sent when you run `ttal task test <uuid>`. The test agent receives this prompt and runs tests for the task.

## Examples

### Custom execute prompt with a specific skill

```toml
[prompts]
execute = "/sp-tdd"
```

This would use test-driven development for all workers instead of the default plan-execution flow.

### Adding project-specific instructions

```toml
[prompts]
execute = """
/sp-executing-plans
Always run `make ci` before committing.
Use conventional commit messages.
"""
```

### Verbose research prompt

```toml
[prompts]
research = """
Research task {{task-id}} thoroughly:
1. Search for existing solutions
2. Compare at least 3 approaches
3. Write findings to ~/clawd/docs/research/
4. Annotate the task with Research: <path>
"""
```
