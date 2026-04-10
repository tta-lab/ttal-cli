---
name: ttal-skill
description: "Manage skills — list, get, and find skills deployed to ~/.agents/skills/"
---

# ttal skill

Manage skills deployed to `~/.agents/skills/`. Skills are synced from `skills/` via `ttal sync`.

## Commands

### List skills
```bash
ttal skill list          # list all skills
ttal skill list --all   # same (shows category column)
ttal skill list --json   # JSON output
```

### Get skill content
```bash
ttal skill get <name>    # print skill content (frontmatter stripped)
ttal skill get <name> --json  # JSON output with content
```

### Find skills
```bash
ttal skill find <keyword>       # search by keyword
ttal skill find debug commit    # multiple keywords (OR match)
```
