---
title: Prompts
description: Customize what gets sent to agents and workers
---

<div v-pre>

ttal lets you customize the prompts sent to agents when routing tasks. This controls what instructions your design, research, test, execute, triage, review, and re-review agents receive.

## Configuration

Create `~/.config/ttal/prompts.toml` with your prompt templates:

```toml
context = """
$ diary {{agent-name}} read
$ ttal agent list
---

## New Task Assignment

$ ttal pipeline prompt
"""

triage = "Execute `skill get triage`\n\nPR review posted. Read {{review-file}}, assess and fix issues."
review = "You are reviewing PR #{{pr-number}} in {{owner}}/{{repo}}."
re_review = "Re-review scope: {{review-scope}}"
```

> **Note:** Prompts live in a dedicated `prompts.toml` file rather than `config.toml`, keeping your main config file focused on settings and team configuration. Role-based prompts (for agents with `role: designer` or `role: researcher`) live in `roles.toml`, not `prompts.toml`. The `coder` role prompt also lives in `roles.toml`.

## Context Template (`context`)

The `context` prompt is the universal CC SessionStart template. It is rendered for every agent session and supports two line types:

- **`$ <cmd>`** — shell command, executed and replaced with its stdout (header: `--- <cmd> ---`)
- **Plain text** — passed through as-is

Template vars `{{agent-name}}` and `{{team-name}}` are expanded before execution.

Commands that fail or produce no output are silently skipped — partial output is fine.

```toml
context = """
$ diary {{agent-name}} read
$ ttal agent list
---

## New Task Assignment

Read the task and do your best based on the context.
Run `ttal task get` (no extra arguments) to retrieve task details.
$ ttal pipeline prompt
"""
```

`ttal pipeline prompt` reads `TTAL_JOB_ID` / `TTAL_AGENT_NAME` from the environment to find the current task and output the role-specific prompt (coder instructions, review prompt, etc.).

## Template variables

- **`{{task-id}}`** — replaced with the task's short UUID at runtime

### Skill References

Skills are referenced via the `skill get` command in prompts. Use `Execute \`skill get <name>\`` literally to instruct the agent to fetch the skill:

```toml
triage = """\
Execute `skill get triage`

PR review posted. Read it, assess and fix issues.
"""
```

> **Note:** Skills are not inlined automatically. Prompts that need skill content tell agents to execute `skill get` explicitly. Manager-plane agents receive skill directives via the `skills` field in `pipelines.toml` stage config.

## Available Prompt Keys

| Key | Used by | Template variables |
|-----|---------|-------------------|
| `context` | CC SessionStart hook (all agents) | `{{agent-name}}`, `{{team-name}}`, `$ cmd` |
| `designer` | `ttal go <uuid>` (agent with `role: designer`) | `{{task-id}}` |
| `researcher` | `ttal go <uuid>` (agent with `role: researcher`) | `{{task-id}}` |
| `coder` | `ttal go` (worker spawn, via `roles.toml`) | `{{task-id}}` |
| `triage` | PR review → coder | `{{review-file}}` |
| `review` | Reviewer initial prompt | `{{pr-number}}`, `{{pr-title}}`, `{{owner}}`, `{{repo}}`, `{{branch}}` |
| `re_review` | Re-review after fixes | `{{review-scope}}`, `{{coder-comment}}` |

## How each prompt is used

### `context`

The universal SessionStart template. Rendered for every agent session via the CC hook.
Lines starting with `$ ` are executed as shell commands. The `$ ttal pipeline prompt` line
outputs the role-specific prompt (coder instructions, review prompt, plan review prompt, etc.)
based on the current task's pipeline stage.

Workers get `TTAL_JOB_ID` derived from their worktree path. Managers and reviewers get
`TTAL_AGENT_NAME` from their `--agent` flag. Both are available as env vars during `$ cmd` execution.

### `coder` (in `roles.toml`)

The coder role prompt is rendered by `ttal pipeline prompt` and injected via the context
template. Configure it in `roles.toml` under `[coder]`.

Default: instructs the coder agent to implement the task using the plan from the task context.

### `designer`

Controls what gets sent when you run `ttal go <uuid>` where the agent has `role: designer`. The agent receives this prompt with the task UUID, reads the task details, and writes an implementation plan.

Default: asks the agent to write a plan document and annotate the task with its path.

### `researcher`

Controls what gets sent when you run `ttal go <uuid>` where the agent has `role: researcher`. The agent receives this prompt, investigates the topic, and writes findings.

Default: asks the agent to research the topic and annotate the task with the findings path.

### `triage`

Sent to the coder window after the reviewer posts a PR review. Contains the review file path for the coder to read and assess.

### `review`

The initial prompt for the reviewer when spawning a new review window. Contains PR metadata (number, title, owner, repo, branch). Rendered by `ttal pipeline prompt` in the reviewer's session.

### `re_review`

Sent to the reviewer when the coder pushes fixes and requests a re-review. Contains the review scope and optional coder comment.

## Examples

### Custom context template with diary and task list

```toml
# ~/.config/ttal/prompts.toml
context = """
$ diary {{agent-name}} read
$ ttal today list
---

$ ttal pipeline prompt
"""
```

### Custom coder role prompt

```toml
# ~/.config/ttal/roles.toml
[coder]
prompt = """Always run `make ci` before committing.
Use conventional commit messages.

Read the task: ttal task get"""
```

### Custom triage prompt

```toml
# ~/.config/ttal/prompts.toml
triage = """\
Execute `skill get triage`

PR review posted.{{review-file}} Read it, assess and fix issues.
Post your triage update with ttal comment add when done."""
```

### Custom review prompt

```toml
# ~/.config/ttal/prompts.toml
review = """\
You are reviewing PR #{{pr-number}} — "{{pr-title}}" in {{owner}}/{{repo}}.
1. Execute `skill get pr-review` to review the diff
2. Post findings with ttal comment add
3. End with VERDICT: LGTM or VERDICT: NEEDS_WORK"""
```

</div>
