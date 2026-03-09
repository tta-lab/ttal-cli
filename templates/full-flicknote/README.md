---
name: Full (FlickNote)
description: Complete team with personalities, plans stored in FlickNote
agents: compass, ink, owl, falcon
install_hint: "Requires FlickNote CLI (coming soon — see https://github.com/tta-lab/flicknote-cli)"
---

# Full (FlickNote) — 4-Agent Team with Personalities

A complete ttal team with creature identities and FlickNote-based plan storage. This is the power-user setup — agents have personality, voice, and character.

## Agents

| Agent | Creature | Role |
|-------|----------|------|
| **compass** | Compass | Task navigator — routes work, manages priorities |
| **ink** | Octopus | Design architect — writes detailed implementation plans |
| **owl** | Owl | Researcher — investigates topics, writes structured findings |
| **falcon** | Falcon | Bug fix designer — diagnoses bugs, writes fix plans |

## Storage

Plans and research are stored in [FlickNote](https://flicknote.app/) — an AI-native note system with MCP integration. This keeps plans separate from code repos and makes them searchable across projects.

```
flicknote add 'plan content' --project plans    # Save a plan
flicknote get <hex-id>                           # Read it back
flicknote get <hex-id> --tree                    # See structure
```

## Quick start

1. Set up your workspace:
   ```bash
   ttal onboard --scaffold full-flicknote
   ```
2. Edit `~/.config/ttal/config.toml`:
   - Set `chat_id` and `team_path`
3. Create 4 Telegram bots via @BotFather (give them matching names!)
4. Add tokens to `~/.config/ttal/.env`:
   ```
   COMPASS_BOT_TOKEN=...
   INK_BOT_TOKEN=...
   OWL_BOT_TOKEN=...
   FALCON_BOT_TOKEN=...
   DEFAULT_NOTIFICATION_BOT_TOKEN=...   # Team notifications (worker status, PR events)
   ```
5. Install FlickNote CLI: see [flicknote.app](https://flicknote.app/)
6. Run `ttal doctor` to verify
7. Start the daemon: `ttal daemon start`

## Why personalities?

Agents with identity produce better work. A "researcher" is generic — an "owl" has a point of view. Personality shapes how agents communicate, what they prioritize, and how they approach problems. It's not cosmetic — it's a design decision.
