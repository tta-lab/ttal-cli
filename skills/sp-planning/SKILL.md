---
name: sp-planning
description: Use when the user asks for a concrete implementation plan for an agreed change before code is written.
---

# Implementation Planning

Write a plan that a skilled developer can execute without rediscovering the design. FlickNote is the single source of truth.

## 1. Explore Reality

Before planning:

- Read the relevant code, tests, docs, and repository rules.
- Search prior work with `flicknote find <keywords>`.
- Map existing constraints, interfaces, dependencies, and failure modes.
- Confirm the target repository.

Do not design from filenames or assumptions.

## 2. Confirm the Approach

Before writing the detailed plan, tell the user:

- What exists now
- The proposed approach and why
- The main trade-off or risk

Stop for explicit alignment if the direction has not already been approved.

## 3. Define the Plan

Include:

- Goal and anti-goals
- Exit criteria expressed as observable behavior
- Exact scope and files
- Ordered implementation stages
- Test strategy and exact verification commands
- Risks, dependencies, and rollout concerns

For behavior changes and bug fixes, plan a TDD sequence:

1. Add a focused test.
2. Run it and confirm the expected failure.
3. Implement the minimum change.
4. Run focused and broader tests.
5. Refactor only while green.

Mechanical deletions, generated files, and configuration-only work do not need invented tests; state the appropriate verification instead.

## 4. Keep Scope Executable

Split the plan into separate deliverable phases when:

- Multiple repos can ship independently
- A stage has a separate rollback boundary
- The plan is too large to review or verify as one change

Record dependencies between phases in the plan. Do not create tasks automatically.

## 5. Persist to FlickNote

Create or update one note in the `orientation` project:

flicknote add --project orientation
flicknote detail <id> --tree
flicknote modify <id>

Use this structure:

# Plan: <title>
## Goal
## Anti-goals
## Current state
## Approach
## Implementation stages
## Test strategy
## Exit criteria
## Risks and dependencies

Each stage must name the files, behavior change, tests, verification, and commit boundary when useful.

## Worker-Eye Check

Re-read the saved note as the implementer:

- Are paths and commands accurate?
- Is each stage self-contained and ordered?
- Are hidden assumptions written down?
- Would execution require another design decision?
- Is the scope small enough to deliver safely?

Update the FlickNote note until the answers are clear. Return its ID and a concise summary. Stop; do not start implementation or invoke another skill.
