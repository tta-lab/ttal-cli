---
name: plan-triage
description: "Triage plan review findings — categorize issues, fix actionable ones, report"
argument-hint: ""
claude-code:
  allowed-tools:
    - Bash
    - Read
    - Glob
    - Grep
---

# Plan Triage

Triage plan review findings: categorize each issue, fix what's actionable, then report.

You already have the plan and its review issues in context. No need to search or re-read — just categorize, fix, and report.

## Usage

```
/plan-triage
```

Run this after `/plan-review` has identified issues. The plans and issues are already in your conversation context.

## Phase 1: Categorize

For each issue from the plan review, categorize it:

**Actionable (fix now)**
- Gaps that would block a worker
- Wrong assumptions verified against the codebase
- Missing error handling or edge cases that matter
- Under-engineered: shortcuts that will create tech debt
- Ambiguities that can be clarified from the codebase

Format: `[FIX] <summary> — <why it matters>`

**False Positive (push back)**
- Assumption that's actually correct (verified)
- Edge case that can't happen in this context
- "Over-engineered" flag on complexity that's genuinely needed

Format: `[FALSE POSITIVE] <summary> — <why it's wrong>`

**Deferrable (follow-up)**
- Nice-to-have improvements, not blockers
- Style preferences in the plan structure
- Over-engineered parts that work fine, just aren't minimal

Format: `[DEFER] <summary> — <why it can wait>`

**Needs Neil (can't resolve without human input)**
- Business requirements or product decisions
- Priority tradeoffs
- Integration points the reviewer can't verify

Format: `[ASK] <summary> — <question for Neil>`

## Phase 2: Fix

Address all `[FIX]` items directly in the plan using flicknote:

```bash
# Update a section
echo "updated content" | flicknote replace <id> --section <section-id>

# Append missing steps
echo "new content" | flicknote append <id>

# Insert before/after a section
echo "content" | flicknote insert <id> --after <section-id>
```

For each fix:
- Make the change in the flicknote plan
- Keep fixes minimal — address the issue, don't redesign the plan
- If a fix requires significant rewriting, recategorize it as `[ASK]` instead

## Phase 3: Report

```markdown
# Plan Triage: <plan title>

## Fixed
- [x] <issue> — <what was changed>

## False Positive
- [FALSE POSITIVE] <issue> — <why>

## Deferred
- [DEFER] <issue> — <why>

## Needs Neil
- [ASK] <issue> — <question>

## Verdict
**Ready / Needs Revision / Needs Rethink**
```

**Verdict rules:**
- **Ready** — all actionable items fixed, no `[ASK]` items remaining
- **Needs Revision** — has `[ASK]` items that need Neil's input before proceeding
- **Needs Rethink** — fundamental problems that small fixes can't address

## Turns

Triage is iterative. After Neil answers `[ASK]` items or a designer revises:

```
plan-review → plan-triage (round 1) → fix + ask
  → Neil answers / designer revises
plan-review → plan-triage (round 2) → fix
  → all clear
plan-triage (round 3) → ready
```

Include the **round number** in the header. Track how the verdict changed:
- **Ready** (was Needs Revision) — issues addressed
- **Needs Revision** (still) — issues weren't addressed
- **Needs Rethink** (was Needs Revision) — revision revealed deeper problems

If a plan is stuck for 2+ rounds, escalate to Neil rather than routing back again.

## Guidelines

- Fix what you can, ask about what you can't — don't block on things you can resolve yourself
- Keep fixes minimal — address the issue, don't redesign the plan
- Be objective — don't approve plans just because you fixed a few things
- Evaluate by impact, not by count — one critical gap beats five minor style issues
