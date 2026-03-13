---
name: ttal-today
description: Manage your daily task focus list using taskwarrior scheduled date.
---

# ttal today

Manage your daily task focus list (uses taskwarrior `scheduled` date).

```bash
ttal today list                # pending tasks scheduled on or before today
ttal today completed           # tasks completed today
ttal today add <uuid>          # set scheduled:today on a task
ttal today remove <uuid>       # clear scheduled date from a task
```
