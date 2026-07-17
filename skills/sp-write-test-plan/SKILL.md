---
name: sp-write-test-plan
description: Use when the user asks for an integration test plan for a multi-component, failure-prone, or historically fragile system.
---

# Integration Test Planning

Write a reasoning-driven test plan in FlickNote. Cover expected behavior and adversarial failure modes.

## Explore Reality

- Read the current implementation and existing tests.
- Search prior designs, research, and bug reports with `flicknote find <keywords>`.
- Map entry points, state transitions, external systems, concurrency, retries, and partial failures.
- Confirm the target repository and exact test surface.

Discuss the proposed test boundary with the user before writing the plan if it is not already agreed.

## Constructive Coverage

Define:

- Major happy-path lifecycles
- Boundary and empty inputs
- Defaults and missing configuration
- State invariants and security preconditions
- Idempotency and resource lifecycle rules

## Adversarial Coverage

Run three passes:

1. Prior failures: turn each relevant historical bug class into a regression test.
2. Seam walk: concurrency, retries, partial failures, idempotency, timeouts, corruption, edge dates/numbers, hostile inputs.
3. Red team: write concrete failure hypotheses and the test that would falsify each one.

Distinguish:

- Planned: test is needed; no proof of failure
- Confirmed broken: code or evidence proves current behavior is wrong
- Dismissed: investigated and ruled out with a reason

## Persist to FlickNote

Save the plan in `orientation`:

flicknote add --project orientation

Structure:

# Test Plan: <component>
## Scope and anti-goals
## Current implementation
## Constructive cases
## Prior-failure regressions
## Seam walk
## Red-team hypotheses
## Test environment and fixtures
## Execution order
## Exit criteria
## Risks and unreachable coverage

Each test case must identify the setup, action, expected result, proof source, and cleanup. Record confirmed bugs in the same note; do not create tasks automatically.

## Validate

Re-read the note as the test implementer:

- Are fixtures and environments obtainable?
- Can each result be attributed to the layer under test?
- Are negative controls actually achievable?
- Are paths, commands, evidence, and cleanup exact?
- Is unreachable third-party coverage stated plainly?

Update the FlickNote note, then return its ID and a concise summary. Stop; do not execute the plan or invoke another skill.
