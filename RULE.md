# ttal Quick Reference

## Messaging

```bash
ttal send --to <agent> "message"
```

## Tasks

```bash
ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"
ttal task get <uuid>                    # rich prompt with inlined docs
ttal task find <keyword>                # search pending tasks
ttal task find <keyword> --completed    # search completed tasks
ttal task design <uuid>                 # route to design agent
ttal task research <uuid>              # route to research agent
ttal task execute <uuid>               # spawn worker
```

## PRs

```bash
ttal pr create "title" --body "description"
ttal pr modify --title "new" --body "new"
ttal pr merge                          # squash merge
ttal pr comment create "markdown"
ttal pr comment list
```

## Projects & Agents

```bash
ttal project list                      # all active projects
ttal project get <alias>               # project details
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
