# ttal Quick Reference

## Messaging

```
cat <<'EOF' | ttal send --to <recipient>
message
EOF

# Worker session
cat <<'EOF' | ttal send --to <job_id>:<agent_name>
message
EOF
```

## Tasks

```
ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"
ttal task get                           # rich prompt with inlined docs
ttal task find <keyword>                # search pending tasks
ttal task find <keyword> --completed    # search completed tasks
ttal go <uuid>                    # advance task through pipeline stage
```

## PRs

```
ttal pr create "title" --body "description"
ttal pr modify --title "new" --body "new"
ttal go <uuid>                          # squash merge
```


## Projects & Agents

```
ttal project list                      # all active projects with paths
ttal agent info <name>                 # agent details
ttal agent list                        # all agents
```



## Sync

```
ttal sync                    # deploy subagents + config TOMLs to runtime dirs
ttal sync --dry-run          # preview what would be deployed
```
Sources: `agents/` → `~/.claude/agents/`
Config TOMLs (prompts.toml, roles.toml, pipelines.toml) are deployed from team_path → `~/.config/ttal/`.

Skills are deployed from `skills/` to `~/.agents/skills/` via `ttal sync`. They are accessed at runtime via `skill get <name>`.

## Task Routing

Route tasks to the right agent instead of doing everything yourself.

```
ttal go <uuid>    # advance task through pipeline stage (route to agent or spawn worker)
```

## Messaging Context

Messages arrive as prefixed text in your input:
- `<- telegram:<name>` — from a human via Telegram
- `<- <agent-name>` — from another agent

Use the same explicit command for humans and agents. The recipient is a human alias, agent name, or worker address (`<uuid>:<agent-name>`). Session output is not delivered passively.

```
cat <<'EOF' | ttal send --to <recipient>
message
EOF
```

**When to reply:**
- Meaningful updates: task complete, blocked, need input, PR ready
- Keep replies concise
- You don't need to reply to every message — use judgement

## Git

```
ttal push                              # push current branch to origin via daemon
ttal push --force                      # force-with-lease; blocked on main/master
ttal tag v1.0.0 --project <alias>      # create + push git tag via daemon
```

- Use `ttal push` for git push — proxied through daemon
- Use `ttal tag` for git tags — creates tag locally and pushes via daemon. `--project` is required.
- Never run `git push` or `git push origin <tag>` directly from a worker session

## Tool Usage

- Never use `run_in_background` for `ttal go` — it completes in seconds and backgrounding causes output read races

## Notes

- ttal is the SSOT for agent identity. Don't hardcode agent info — query ttal.


## Status
Review complete — 2 findings.
ENDBASH
```
