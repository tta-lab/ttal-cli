# Agent Resources

This directory contains skills, subagents, and commands used with ttal.

## Structure

```
docs/
├── skills/       # Methodology skills (sp-planning, sp-tdd, etc.)
├── agents/       # Subagent definitions (pr-reviewers, task-creator, etc.)
└── commands/     # Static commands (tell-me-more, update-claude-md)
```

## How it works

**Subagents** are deployed via `ttal sync`:

```bash
ttal sync            # Deploy subagents and rules to runtime
ttal sync --dry-run  # Preview what would be deployed
```

Your `config.toml` `[sync]` section tells ttal where to find these:

```toml
[sync]
  subagents_paths = ["./docs/agents"]
```

**Skills** are stored in flicknote. Use `ttal skill import` to upload:

```bash
ttal skill import docs/skills --apply
ttal skill import docs/commands --apply --category command
```

## Adding custom skills

Create a new directory in `docs/skills/` with a `SKILL.md` file. See existing skills for the format.
Then import: `ttal skill import docs/skills --apply`

## Included

### Skills
| Skill | Purpose |
|-------|---------|
| sp-planning | Write detailed implementation plans |
| sp-research | Structured research methodology |
| sp-brainstorming | Collaborative idea exploration and design |
| sp-tdd | Test-driven development workflow |
| sp-verify | Verify work before claiming completion |
| sp-debugging | Systematic debugging approach |
| git-omz | Git operations via oh-my-zsh aliases |
| taskwarrior | Task management integration |

### Subagents
| Agent | Purpose |
|-------|---------|
| task-creator | Create taskwarrior tasks with proper conventions |
| task-deleter | Safely delete tasks |
| pr-code-reviewer | Code quality review |
| pr-code-simplifier | Simplify code for clarity |
| pr-comment-analyzer | Analyze code comments |
| pr-silent-failure-hunter | Find suppressed errors |
| pr-test-analyzer | Review test coverage |
| pr-type-design-analyzer | Analyze type design quality |

## Acknowledgments

The `sp-*` skills (superpowers) are based on [obra/superpowers](https://github.com/obra/superpowers) by Jesse Vincent. Adapted for ttal's agent workflow with task annotations, flicknote storage, and multi-agent routing.
