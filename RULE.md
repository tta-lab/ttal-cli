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

## Notes & Pipeline

```
flicknote add "content" --project orientation
flicknote find <keyword>
flicknote detail <id>
ttal go <uuid>                          # advance an existing pipeline job
```

## PRs

```
echo "description" | og pr create "title"
echo "new" | og pr modify --title "new"
ttal go <uuid>                          # squash merge
```


## Projects & Agents

```
project list                           # all active projects with paths
ttal agent info <name>                 # agent details
ttal agent list                        # all agents
```



## Sync

```
ttal sync                    # deploy subagents, rules, and skills to runtime dirs
ttal sync --dry-run          # preview what would be deployed
```
Sources: `agents/` → `~/.claude/agents/`
TTAL config TOMLs are managed outside `ttal sync` so Nix/home-manager can own `~/.config/ttal/`.

Skills are deployed from `skills/` to `~/.agents/skills/` via `ttal sync`. They are accessed at runtime via `skill get <name>`.

## Task Routing

Route tasks to the right agent instead of doing everything yourself.

```
ttal go <uuid>    # advance task through pipeline stage (route to agent or spawn worker)
```

## Messaging Context

Messages arrive as prefixed text in your input:
- `<- <name>:telegram` — from a human via Telegram
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
og git push                            # push current branch to origin
og git push --force                    # force-with-lease; blocked on main/master
og git tag v1.0.0                      # create + push git tag
```

- Use `og git push` for git push
- Use `og git tag` for git tags
- Never run `git push` or `git push origin <tag>` directly from a worker session

## Tool Usage

- Never use `run_in_background` for `ttal go` — it completes in seconds and backgrounding causes output read races

## Notes

- ttal is the SSOT for agent identity. Don't hardcode agent info — query ttal.


## Status
Review complete — 2 findings.
ENDBASH
```
