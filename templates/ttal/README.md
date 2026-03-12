# templates/ttal — Author's Real Team Config

This is the author's production agent team configuration. Unlike the other templates
(`basic`, `full-markdown`, `full-flicknote`) which are starter scaffolds, this is a
living showcase of how a real team is structured and managed with ttal.

## The Team

7 professional agents, each with a dedicated workspace and CLAUDE.md:

| Agent | Role | Creature |
|-------|------|---------|
| **Yuki** 🐱 | Task orchestrator — creates, routes, and manages work via taskwarrior | Black Cat |
| **Athena** 🦉 | Researcher — conducts multi-source deep dives, writes findings to flicknote | Owl |
| **Inke** 🐙 | Design architect — writes executable implementation plans from research | Octopus |
| **Kestrel** 🦅 | Bug fix designer — diagnoses root causes and writes fix plans | Kestrel |
| **Eve** 🦘 | Agent creator — designs new agent identities, handles respawn updates | Kangaroo |
| **Quill** 🐦‍⬛ | Skill design partner — helps create well-designed, shareable Claude Code skills | Crow |
| **Lyra** 🦎 | Communications writer — polishes outward-facing text, adapts tone per platform | Lizard |

## How It Differs from Starter Templates

The starter templates (`basic`, `full-markdown`, `full-flicknote`) give you a minimal
scaffold to customize. This template shows a fully built-out team:

- Each agent has a complete CLAUDE.md with personality, role, tools, and workflow
- `config.toml` points sync paths to the shared `../docs/` directory
- Prompt templates use flicknote for plan/research storage (`ttal.plans`, `ttal.research`)
- The design/research/execute/review pipeline is fully wired up

## Structure

```
templates/ttal/
├── config.toml          # Team config — sync paths, prompts, agent roles
├── README.md            # This file
├── yuki/CLAUDE.md       # Task orchestrator
├── athena/CLAUDE.md     # Researcher
├── inke/CLAUDE.md       # Design architect
├── kestrel/CLAUDE.md    # Bug fix designer
├── eve/CLAUDE.md        # Agent creator
├── quill/CLAUDE.md      # Skill design partner
└── lyra/CLAUDE.md       # Communications writer
```

Shared skills, subagents, and commands live in `templates/docs/` — referenced via
`../docs/` in `config.toml`. All agents in this template use that shared library.

## Adopting This Template

To use this team as your starting point:

1. Copy `templates/ttal/` to your workspace directory (e.g. `~/clawd/`)
2. Update `config.toml`: set `team_path` and `chat_id` for your setup
3. Update agent CLAUDE.md files: replace `/Users/neil/clawd/` paths with your own
4. Run `ttal sync` to deploy skills, subagents, and commands to Claude Code's runtime dirs

See the [ttal documentation](../../README.md) for full setup instructions.
