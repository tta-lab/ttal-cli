# Agent Resources

This directory contains skills, subagents, and commands that `ttal sync` deploys to your agent runtimes.

## Structure

```
docs/
├── skills/       # Methodology skills (sp-planning, sp-tdd, etc.)
├── agents/       # Subagent definitions (pr-reviewers, task-creator, etc.)
└── commands/     # Slash commands (pr-review, triage, etc.)
```

## How it works

Run `ttal sync` to deploy these to your agent runtime (Claude Code or Codex):

```bash
ttal sync            # Deploy to runtime
ttal sync --dry-run  # Preview what would be deployed
```

Your `config.toml` `[sync]` section tells ttal where to find these:

```toml
[sync]
  skills_paths = ["./docs/skills"]
  subagents_paths = ["./docs/agents"]
  commands_paths = ["./docs/commands"]
```

## Adding custom skills

Create a new directory in `docs/skills/` with a `SKILL.md` file. See existing skills for the format.

## Included

### Skills
| Skill | Purpose |
|-------|---------|
| sp-planning | Write detailed implementation plans |
| sp-executing-plans | Execute plans step by step |
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

### Commands
| Command | Purpose |
|---------|---------|
| pr-review | Comprehensive PR review |
| triage | Triage PR review comments |
| task-create | Create tasks from plans |
| tell-me-more | Elaborate on a concept |

## Acknowledgments

The `sp-*` skills (superpowers) are based on [obra/superpowers](https://github.com/obra/superpowers) by Jesse Vincent. Adapted for ttal's agent workflow with task annotations, flicknote storage, and multi-agent routing.
