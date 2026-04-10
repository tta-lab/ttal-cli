# ttal Quick Reference

## Messaging

```bash
ttal send --to <agent> "message"
ttal send --to <job_id>:<agent_name> "message"   # send to worker session
```

## Tasks

```bash
ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"
ttal task get                           # rich prompt with inlined docs
ttal task find <keyword>                # search pending tasks
ttal task find <keyword> --completed    # search completed tasks
ttal go <uuid>                    # advance task through pipeline stage
```

## PRs

```bash
ttal pr create "title" --body "description"
ttal pr modify --title "new" --body "new"
ttal go <uuid>                          # squash merge
```


## Projects & Agents

```bash
ttal project list                      # all active projects with paths
ttal agent info <name>                 # agent details
ttal agent list                        # all agents
```

## Today

```bash
ttal today list                        # tasks scheduled today
ttal today add <uuid>                  # schedule for today
ttal today completed                   # done today
```

## Voice

```bash
ttal voice speak "text"                # speak with your voice
ttal voice speak "text" --voice <id>   # specific voice
ttal voice status                      # check server
```

## Sync

```bash
ttal sync                    # deploy subagents + config TOMLs to runtime dirs
ttal sync --dry-run          # preview what would be deployed
```
Sources: `agents/` → `~/.claude/agents/`
Config TOMLs (prompts.toml, roles.toml, pipelines.toml) are deployed from team_path → `~/.config/ttal/`.

Skills are deployed from `skills/` to `~/.agents/skills/` via `ttal sync`. They are accessed at runtime via `ttal skill get <name>`.

## Task Routing

Route tasks to the right agent instead of doing everything yourself.

```bash
ttal go <uuid>    # advance task through pipeline stage (route to agent or spawn worker)
```

## Messaging Context

Messages arrive as prefixed text in your input:
- `[telegram from:<name>]` — from a human via Telegram
- `[agent from:<name>]` — from another agent

**Replying to humans (Telegram):** Just output text naturally. The bridge picks up your response and delivers it to Telegram automatically. Don't use `ttal send` for this.

**When to reply:**
- Meaningful updates: task complete, blocked, need input, PR ready
- Keep replies concise
- You don't need to reply to every message — use judgement

## Git

```bash
ttal push                              # push current branch to origin via daemon
ttal tag v1.0.0 --project <alias>      # create + push git tag via daemon
```

- Use `ttal push` for git push — proxied through daemon
- Use `ttal tag` for git tags — creates tag locally and pushes via daemon. `--project` is required.
- Never run `git push` or `git push origin <tag>` directly from a worker session

## Tool Usage

- Never use `run_in_background` for `ttal go` — it completes in seconds and backgrounding causes output read races

## Notes

- ttal is the SSOT for agent identity. Don't hardcode agent info — query ttal.
