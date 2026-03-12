---
name: update-claude-md
description: "Update CLAUDE.md based on what was just discussed in conversation"
claude-code: {}
opencode: {}
---

# Update CLAUDE.md

## Overview

After a meaningful conversation — decisions made, patterns discovered, preferences stated — the user may ask you to capture what was discussed into the project's CLAUDE.md. This ensures future sessions benefit from today's context.

## Which CLAUDE.md?

Find the CLAUDE.md you own in the current repo:

- **In `~/clawd/`** — update your agent file (e.g. `quill/CLAUDE.md`, `kestrel/CLAUDE.md`)
- **In any other repo** — update the project root `CLAUDE.md`

If unsure, ask which file to update.

## How to Update

1. **Review the conversation** — identify decisions, preferences, patterns, or conventions that were established
2. **Read the current CLAUDE.md** — understand what's already documented
3. **Add only what's new** — don't duplicate existing content
4. **Place it in the right section** — match the file's existing structure. Create a new section only if nothing fits.
5. **Be concise** — write it as an instruction, not a narrative. Future agents need to act on it, not read a story.

## What to Capture

- Workflow decisions ("always use X instead of Y")
- Tool preferences ("use ttal pr, not gh")
- Conventions discovered ("tests go in __tests__/ not test/")
- Architecture decisions ("we chose X because Y")
- Gotchas and warnings ("don't run Z, it breaks W")

## What NOT to Capture

- Session-specific details (current task, in-progress work)
- Anything already in CLAUDE.md (check first!)
- Speculative conclusions from one conversation
- Temporary workarounds (unless they'll persist)

## After Updating

- Show the user what you added/changed
- Commit if asked (follow the repo's commit conventions)
