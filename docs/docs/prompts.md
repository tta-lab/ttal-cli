---
title: Prompts
description: Customize what gets sent to agents and workers
---

<div v-pre>

ttal lets you customize the prompts sent to agents when routing tasks. This controls what instructions your design, research, test, execute, triage, review, and re-review agents receive.

## Configuration

Create `~/.config/ttal/prompts.toml` with your prompt templates:

```toml
execute = "{{skill:sp-executing-plans}}"
triage = "{{skill:triage}}\nPR review posted. Read {{review-file}}, assess and fix issues."
review = "You are reviewing PR #{{pr-number}} in {{owner}}/{{repo}}."
re_review = "Re-review scope: {{review-scope}}"
```

> **Note:** Prompts live in a dedicated `prompts.toml` file rather than `config.toml`, keeping your main config file focused on settings and team configuration. Role-based prompts (for agents with `role: designer` or `role: researcher`) live in `roles.toml`, not `prompts.toml`.

## Template variables

- **`{{task-id}}`** — replaced with the task's short UUID at runtime
- **`{{skill:name}}`** — replaced with the runtime-appropriate skill invocation (see below)

### Skill References

Use `{{skill:name}}` to reference skills in prompts. This resolves to the
correct invocation syntax based on the target agent's runtime:

| Runtime     | `{{skill:triage}}` resolves to |
|-------------|-------------------------------|
| claude-code | `/triage`                     |
| codex       | `$triage`                     |

Skill references should appear at the **beginning** of the prompt (first line),
as all runtimes require skill invocations at the start of the message.

## Available Prompt Keys

| Key | Used by | Template variables |
|-----|---------|-------------------|
| `designer` | `ttal go <uuid>` (agent with `role: designer`) | `{{task-id}}`, `{{skill:name}}` |
| `researcher` | `ttal go <uuid>` (agent with `role: researcher`) | `{{task-id}}`, `{{skill:name}}` |
| `execute` | `ttal go` | `{{task-id}}`, `{{skill:name}}` |
| `triage` | PR review → coder | `{{review-file}}`, `{{skill:name}}` |
| `review` | Reviewer initial prompt | `{{pr-number}}`, `{{pr-title}}`, `{{owner}}`, `{{repo}}`, `{{branch}}`, `{{skill:name}}` |
| `re_review` | Re-review after fixes | `{{review-scope}}`, `{{coder-comment}}`, `{{skill:name}}` |

## How each prompt is used

### `execute`

The execute prompt is prepended to the worker's spawn prompt. When you run `ttal go <uuid>`, the worker receives this prompt followed by the task context (description, annotations, inlined docs).

Default: invokes the executing-plans skill to implement the task step by step.

### `designer`

Controls what gets sent when you run `ttal go <uuid>` where the agent has `role: designer`. The agent receives this prompt with the task UUID, reads the task details, and writes an implementation plan.

Default: asks the agent to write a plan document and annotate the task with its path.

### `researcher`

Controls what gets sent when you run `ttal go <uuid>` where the agent has `role: researcher`. The agent receives this prompt, investigates the topic, and writes findings.

Default: asks the agent to research the topic and annotate the task with the findings path.

### `triage`

Sent to the coder window after the reviewer posts a PR review. Contains the review file path for the coder to read and assess.

### `review`

The initial prompt for the reviewer when spawning a new review window. Contains PR metadata (number, title, owner, repo, branch).

### `re_review`

Sent to the reviewer when the coder pushes fixes and requests a re-review. Contains the review scope and optional coder comment.

## Examples

### Custom execute prompt with a specific skill

```toml
# ~/.config/ttal/prompts.toml
execute = "{{skill:sp-tdd}}"
```

This would use test-driven development for all workers instead of the default plan-execution flow.

### Adding project-specific instructions

```toml
# ~/.config/ttal/prompts.toml
execute = """\
{{skill:sp-executing-plans}}
Always run `make ci` before committing.
Use conventional commit messages."""
```

### Custom researcher role prompt

```toml
# ~/.config/ttal/roles.toml
researcher = """\
{{skill:tell-me-more}}
Research task {{task-id}} thoroughly:
1. Search for existing solutions
2. Compare at least 3 approaches
3. Write findings to ~/clawd/docs/research/
4. Annotate the task with Research: <path>"""
```

### Custom triage prompt

```toml
# ~/.config/ttal/prompts.toml
triage = """\
{{skill:triage}}
PR review posted.{{review-file}} Read it, assess and fix issues.
Post your triage update with ttal comment add when done."""
```

### Custom review prompt

```toml
# ~/.config/ttal/prompts.toml
review = """\
You are reviewing PR #{{pr-number}} — "{{pr-title}}" in {{owner}}/{{repo}}.
1. Run {{skill:pr-review}} to review the diff
2. Post findings with ttal comment add
3. End with VERDICT: LGTM or VERDICT: NEEDS_WORK"""
```

</div>
