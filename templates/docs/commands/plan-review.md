---
name: plan-review
description: "Review a flicknote plan for issues before approving execution"
argument-hint: "<flicknote-id> [flicknote-id...]"
claude-code:
  allowed-tools:
    - Bash
    - Read
    - Glob
    - Grep
    - Agent
opencode: {}
---

# Plan Review

Read a plan from flicknote and review it for issues before it gets executed by a worker.

## Usage

```
/plan-review <flicknote-id>
/plan-review <id1> <id2> <id3>
```

## Single vs Multiple Plans

**One plan** — review it yourself, in one pass. No subagents needed.

**Multiple plans** — launch a subagent per plan in parallel using the Agent tool. Each subagent gets the full review criteria below and reviews one plan independently. Aggregate the results when all subagents complete.

## Workflow

### 1. Load the plan

```bash
flicknote get <id>
```

If the plan references a project, also check the codebase exists:
```bash
ls <project-path>
```

### 2. Review for issues

Check the plan against these categories:

**Gaps** — missing steps that a worker would need to figure out on their own
- Are all files/modules mentioned actually specified?
- Are dependencies or prerequisites listed?
- Is the order of operations clear?

**Ambiguities** — steps that could be interpreted multiple ways
- Vague language ("improve", "clean up", "fix" without specifics)
- Steps that reference "the right approach" without saying what it is
- Missing acceptance criteria — how does the worker know they're done?

**Wrong assumptions** — things the plan assumes that may not be true
- Use Glob/Grep to verify files and functions the plan references actually exist
- Check if APIs or interfaces match what the plan describes
- Verify the plan's understanding of current code structure

**Missing edge cases** — scenarios the plan doesn't account for
- Error handling
- Backward compatibility
- Concurrent/parallel concerns
- What happens if a step fails midway?

**Structure** — is the plan well-organized architecturally?
- Does the step breakdown match the actual module/file boundaries?
- Are responsibilities split cleanly or is one step doing too much?
- Does the proposed file/folder structure make sense for the codebase?
- Are changes sequenced so each step builds on the last?

**Code logic** — does the proposed approach actually work?
- Will the algorithm/strategy solve the stated problem?
- Are there logical flaws in the implementation steps?
- Do the proposed data flows make sense end-to-end?
- Are there simpler approaches the plan missed?

**Assumptions needing human confirmation** — things only Neil can verify
- Business requirements or product decisions the plan takes as given
- Priority tradeoffs (e.g. "we chose X over Y" — is that still true?)
- Integration points with systems the reviewer can't verify
- Anything the plan states as fact that isn't verifiable from the codebase

Flag these explicitly — don't approve a plan that rests on unconfirmed assumptions.

**Scope issues** — plan is too big or too small
- Could this be split into smaller plans?
- Is the plan trying to do too many things at once?
- Are there tasks mixed in that should be separate?

**Engineering calibration** — is the plan over-engineered, under-engineered, or just right?

Every plan must be classified as one of:
- **Over-engineered** — too many abstractions, premature generalization, unnecessary layers. Building for hypothetical future requirements instead of the current task.
- **Under-engineered** — missing error handling, no tests mentioned, shortcuts that will create tech debt, skipping validation at system boundaries.
- **Just right** — minimum complexity for the current task. Solves the stated problem without adding unnecessary structure.

Justify the classification explicitly. Examples:
- Over: "Plan introduces a plugin system for what's currently a one-off script"
- Under: "Plan modifies a public API but has no migration step for existing callers"
- Right: "Plan adds a new endpoint with input validation, tests, and docs — nothing more"

### 3. Present findings

Output a structured review:

```markdown
# Plan Review: <plan title>

## Issues Found

### Critical (blocks execution)
- <issue> — <why it blocks a worker>

### Important (should fix before executing)
- <issue> — <suggestion>

### Minor (nice to fix)
- <issue> — <suggestion>

## Verification

- [ ] Referenced files exist
- [ ] Step order makes sense
- [ ] Each step is actionable (worker won't have to guess)
- [ ] Proposed logic/approach is sound
- [ ] Structure matches codebase conventions
- [ ] No unconfirmed assumptions (or flagged for Neil)
- [ ] Acceptance criteria are clear

## Engineering Calibration

**Over-engineered / Under-engineered / Just right** — <one sentence justification>

## Verdict

**Ready / Needs revision / Needs rethink**
```

### 4. Wait for decision

Don't modify the plan yourself. Present the review and let Neil or the routing agent decide:
- **Ready** — plan can proceed to `ttal task execute`
- **Needs revision** — route back to the design agent with the issues
- **Needs rethink** — plan has fundamental problems, needs brainstorming first

## Multiple Plans (subagent mode)

When reviewing 2+ plans at once, launch one subagent per plan:

```
For each flicknote ID:
  → Agent(prompt: "Review this plan using the plan-review criteria: <full criteria below>. flicknote get <id>")
```

Launch all subagents in parallel. When they complete, present a summary table:

```markdown
# Plan Review Summary

| Plan | Calibration | Verdict | Critical Issues |
|------|------------|---------|-----------------|
| <id> <title> | Just right | Ready | 0 |
| <id> <title> | Over-engineered | Needs revision | 2 |
```

Then show each plan's full review below the table.

## Turns

Plans rarely pass on the first round. The expected cycle is:

```
plan-review → issues found → route to designer → designer revises → plan-review again → ...
```

Or within a triage batch:
```
plan-triage → plan-review (per plan) → some need revision → route back → plan-triage again → ...
```

After each round, include the **round number** in your review header:

```markdown
# Plan Review: <title> (Round 2)
```

Compare against the previous round's issues:
- **Resolved** — issue from last round is fixed
- **Persisting** — issue was flagged but not addressed
- **New** — issue introduced by the revision

If the same critical issue persists for 2+ rounds, escalate to Neil rather than routing back to the designer again.

## What NOT to do

- Don't rewrite the plan — you're a reviewer, not an author
- Don't execute the plan — that's a separate step
- Don't skip the codebase verification — `Glob`/`Grep` to check references
