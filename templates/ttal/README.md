# templates/ttal — Author's Real Team Config

This is the author's production agent team configuration. Unlike the other templates
(`basic`, `full-markdown`, `full-flicknote`) which are starter scaffolds, this is a
living showcase of how a real team is structured and managed with ttal.

## The Team

11 professional agents, each with a dedicated workspace and CLAUDE.md:

| Agent | Role | Creature/Object |
|-------|------|----------------|
| **Yuki** 🐱 | Task orchestrator — creates, routes, and manages work via taskwarrior | Black Cat |
| **Athena** 🦉 | Researcher — conducts multi-source deep dives, writes findings to flicknote | Owl |
| **Inke** 🐙 | Design architect — writes executable implementation plans (ttal domain) | Octopus |
| **Kestrel** 🦅 | Bug fix designer — diagnoses root causes and writes fix plans | Kestrel |
| **Eve** 🦘 | Agent creator — designs new agent identities, handles respawn updates | Kangaroo |
| **Lyra** 🦎 | Communications writer — polishes outward-facing text, adapts tone per platform | Lizard |
| **Mira** 🧭 | Design architect — writes implementation plans (fb3/Guion domain) | Compass |
| **Nyx** 🔭 | Researcher — deep dives on Guion/fb3 projects and Effect.ts stack | Telescope |
| **Lux** 🔥 | Bug fix designer — diagnoses root causes across all projects | Matchstick |
| **Astra** 📐 | Design architect — writes implementation plans (Effect.ts/fb3 domain) | Drafting Compass |
| **Cael** ⚓ | Devops design architect — K8s, GitOps, Tanka, Flux, infrastructure | Anchor |

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
├── config.toml          # sync paths, voice, team settings
├── prompts.toml         # worker-plane: execute, review, triage
├── roles.toml           # manager-plane: designer, researcher, fixer, manager
├── README.md            # This file
├── yuki/CLAUDE.md       # Task orchestrator
├── athena/CLAUDE.md     # Researcher (ttal domain)
├── inke/CLAUDE.md       # Design architect (ttal domain)
├── kestrel/CLAUDE.md    # Bug fix designer
├── eve/CLAUDE.md        # Agent creator
├── lyra/CLAUDE.md       # Communications writer
├── mira/CLAUDE.md       # Design architect (fb3/Guion domain)
├── nyx/CLAUDE.md        # Researcher (Guion/fb3 domain)
├── lux/CLAUDE.md        # Bug fix designer
├── astra/CLAUDE.md      # Design architect (Effect.ts/fb3 domain)
└── cael/CLAUDE.md       # Devops design architect
```

Shared skills, subagents, and commands live in `templates/docs/` — referenced via
`../docs/` in `config.toml`. All agents in this template use that shared library.

## Adopting This Template

To use this team as your starting point:

1. Copy `templates/ttal/` to your workspace directory
2. Update `config.toml`: set `team_path` and `chat_id` for your setup
3. Run `ttal sync` to deploy skills, subagents, and commands to Claude Code's runtime dirs

See the [ttal documentation](../../README.md) for full setup instructions.
