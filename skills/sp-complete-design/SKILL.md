---
name: sp-complete-design
description: Use when the user asks to finalize and hand back an existing design or implementation plan stored in FlickNote.
---

# Complete a Design

Finalize an existing FlickNote design or plan without starting review or implementation.

## 1. Load the Note

Read the full note and its structure:

flicknote detail <id>
flicknote detail <id> --tree

The note must live in the `orientation` project. Move it there if needed.

## 2. Review as the Implementer

Check:

- Could someone execute it without asking for missing context?
- Are file paths, commands, expected results, and ordering accurate?
- Are goal, anti-goals, exit criteria, tests, risks, and dependencies clear?
- Does any stage still hide a design decision?
- Should an oversized plan be split into deliverable phases?

Write fixes directly back with `flicknote modify`, `replace`, `append`, or section operations.

## 3. Resolve Missing Context

Search the repository, prior notes, or external sources for facts that do not require user judgment. Fold useful findings into the note.

Ask the user only for decisions that materially change scope, behavior, or trade-offs. Persist the answer in FlickNote before continuing.

## 4. Final Check

Re-read the updated note from top to bottom. Confirm that:

- The note is the single source of truth
- No required work is stored only in chat
- No Taskwarrior task, annotation, or task tree is required
- No unresolved question blocks execution

## 5. Hand Back

Return:

- FlickNote ID
- One paragraph describing what and why
- Major stages
- Remaining risks, if any

Keep the summary under 200 words. Stop there. Do not invoke review, `ttal go`, `goal-impl`, or any other next phase.
