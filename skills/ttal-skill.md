---
name: ttal-skill
description: "Manage the skill registry — list, get, find, add, remove, and import skills"
---

# ttal skill

Manage skills stored in flicknote via the skill registry.

## Commands

### List skills
```bash
ttal skill list          # list skills for current agent
ttal skill list --all    # list all skills
```

### Get skill content
```bash
ttal skill get <name>    # print skill content from flicknote
```

### Find skills
```bash
ttal skill find <keyword>       # search by keyword
ttal skill find debug commit    # multiple keywords (OR match)
```

### Add a skill
```bash
ttal skill add <name> <flicknote-id>                    # register existing note
ttal skill add <name> --file path/to/skill.md           # upload file and register
ttal skill add <name> --file skill.md --category tool   # with category
```

### Remove a skill
```bash
ttal skill remove <name>    # unregister (flicknote note NOT deleted)
```

### Import skills from folder
```bash
ttal skill import <folder>                             # dry run
ttal skill import <folder> --apply                     # upload and register
ttal skill import <folder> --apply --force             # re-upload existing
ttal skill import <folder> --apply --category command  # override category
```

Supports two layouts: `<name>/SKILL.md` (directory) and `<name>.md` (flat file).
Category auto-detection: `sp-*` directories → `methodology`, other directories → `tool`, flat files → `reference`.
