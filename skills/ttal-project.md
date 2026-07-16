---
name: ttal-project
description: Manage projects in your team.
---

# Project commands

Read-only project management. Writes go directly to `~/.config/ttal/projects.toml`.

Use the standalone `project` CLI:

```
project list                    # table with alias, org, name
project list --json             # full JSON with k8s fields
project get <alias>             # get path by alias
project get --json <alias>      # full project JSON
project resolve <alias|path>    # resolve alias or path
project jump <alias|org/repo>   # print filesystem path for cd
```
