---
name: sp-debugging
description: Use when a bug needs root-cause diagnosis and a written fix plan before implementation.
---

# Bug Diagnosis and Fix Planning

Diagnose from evidence, then persist the diagnosis and fix plan in FlickNote. Do not implement the fix.

## 1. Confirm the Symptom

Restate the observed behavior, expected behavior, affected surface, and known reproduction. Ask one focused question if the report is ambiguous.

## 2. Find the Root Cause

1. Reproduce the failure.
2. Read the complete errors and logs.
3. Check recent changes and working examples.
4. Trace bad state backward across component boundaries.
5. Form one hypothesis and test it with the smallest useful probe.

If three hypotheses fail, stop and question the model or architecture instead of stacking guesses.

Useful references in this directory:

- `root-cause-tracing.md`
- `defense-in-depth.md`
- `condition-based-waiting.md`

## 3. Confirm the Diagnosis

Tell the user:

- Root cause and supporting evidence
- Proposed fix boundary
- Main risk or trade-off

Get alignment before writing a detailed plan when the fix direction is not already approved.

## 4. Write the FlickNote

Save one note in `orientation`:

flicknote add --project orientation

Structure:

# Fix: <title>
## Symptom
## Root cause
## Evidence and reproduction
## Fix strategy
## Anti-goals
## Implementation stages
## Test strategy
## Exit criteria
## Risks

For behavior fixes, include TDD explicitly:

1. Add a focused regression test.
2. Run it and confirm it fails for the diagnosed reason.
3. Implement the minimum fix.
4. Run focused and broader tests.
5. Refactor while green.

Each stage must name exact files, commands, expected results, and dependencies.

## 5. Worker-Eye Check

Re-read the note as the implementer. Fix missing paths, hidden assumptions, unclear ordering, and unresolved decisions with FlickNote edits.

Return the FlickNote ID and a short diagnosis summary. Stop; do not start implementation or invoke another skill.
