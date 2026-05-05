---
name: sp-write-test-plan
description: Use when authoring an integration test plan before implementation — when about to design tests for a multi-component feature, when probing for already-broken behavior, when a domain has a +bugfix history worth learning from
category: methodology
---

# Test Plan Authoring

## Overview

Test plans are reasoning documents, not exhaustive coverage lists. Two halves: constructive (happy paths, edge cases, invariants) and adversarial (find what is already broken). The constructive half is well-trodden work. The adversarial half earns the keep — it surfaces bugs that exist in the current implementation, not the ones you would add later.

Assume the worker is a skilled test writer, but knows almost nothing about the domain. Document every file they need to test, every prior bug class they should check, every seam that could break.

**Announce at start:** "I am using sp-write-test-plan to author the integration test plan."

**First action:** Run `ttal project list` to identify the target project before writing anything.

## When to Use

Use this skill when you are about to write integration tests and any of these are true:

- The feature crosses component boundaries (async, retries, idempotency, events)
- The domain has prior +bugfix tasks with lessons worth learning from
- You want an adversarial pre-impl pass — find what is already broken before you confirm it works
- The implementation involves concurrent state, external services, timeouts, or partial failures
- Someone on the team said "we should write tests for this"

Do NOT use this skill for unit-level TDD loops (use sp-tdd for RED-GREEN-REFACTOR). Do NOT use this skill as a post-hoc code review (sp-review-against-plan covers that).

## Project Scope Gate

**Rule: 1 task to 1 plan to 1 project or repo.**

Before writing any test plan, confirm the target project:

1. **Run `ttal project list`** — see all available projects
2. **Check the task project field** — if it has one, use it as a hint for the target alias
3. **Validate the repo exists** — run `ttal project get <alias>` to confirm the path

**Hard rule:** Do NOT proceed past this gate without a confirmed single target repo.

If the task touches multiple repos, stop and flag it: a single test plan must cover a single project. Split the task into per-repo test plans.

## Phase 1: Explore Reality

BEFORE writing any test plan, understand what exists. Do not design tests in the abstract — read the implementation.

1. **Read implementation files** — find and read the files and modules under test. Understand data flow, entry points, error paths, dependencies.
2. **Read related task tree** — `task <uuid> tree` — understand what steps the implementation went through, what decisions were made.
3. **Read orientation flicknotes** — `flicknote find <keywords>` — search for orientation docs, design docs, research notes related to this domain.
4. **Read prior bugfix history** — `task +bugfix project:<alias> status:completed export` — extract descriptions of every past bug in this project.
5. **Map test surface** — what are the integration boundaries? External services, databases, file systems, network calls, async queues. List them.

### Red Flags — STOP and Investigate

- You are writing a test plan for code you have not read — read it first
- The implementation references services, configs, or dependencies you do not understand — ask or read more
- You find recent changes that conflict with the task assumptions — reconcile before planning tests
- Prior bugfix history shows a pattern you cannot explain — investigate before including in adversarial pass

## Checkpoint 1: Discuss Approach

After Phase 1, talk through what you found before designing the test plan. Do not go silent and start writing — discuss first.

**Conversational checkpoint:**
- State what you found: implementation patterns, integration boundaries, prior bug classes, any surprises
- Propose your approach: key areas to test, which seams look most fragile, which prior bug classes the current impl might still be vulnerable to
- Ask for alignment: "Does this test approach make sense, or should I consider something different?"
- Keep it lightweight: 3-5 sentences of understanding plus proposed approach plus a question
- **If not aligned** — revise approach and discuss again. Do not proceed to Phase 2 without explicit agreement.

**STOP here.** End your message after presenting your findings and question. Do not begin Phase 2 or write the test plan until the human responds and confirms alignment.

## Phase 2: Constructive

Author the constructive section of the test plan. Group by area: per command, per service, per data flow.

### Happy Paths

For each major usage scenario:
- Full lifecycle: create, read, update, delete
- Successful response under normal conditions
- Idempotent replay of safe operations

### Edge Cases

For each entry point and data path:
- Empty inputs (empty strings, zero values, nil slices)
- Boundary values (max, min, near-overflow, near-timeout)
- Missing optional fields
- Default behavior when config is absent
- Concurrent access to shared state
- Duplicate requests and racing operations

### Invariants and Preconditions

Document what must always be true before and after each operation:
- State invariants (e.g., total consumed does not exceed quota at all times)
- Data consistency rules (e.g., subscription status transitions are monotonic)
- Security preconditions (e.g., unauthenticated requests return 401)
- Resource lifecycle rules (e.g., a closed subscription cannot be reopened)

## Phase 3: Adversarial

Three sequenced passes, executed in order. Each pass produces a table. Do NOT skip pass gamma — the empirical anchor is the most valuable pass.

### Pass gamma: Lessons-from-Prior-Bugs

1. Collect prior bugs: `task +bugfix project:<alias> status:completed export`
2. For each bug, extract the bug class:
   - Race condition
   - Timeout or deadline exceeded
   - Data corruption (stale read, partial write, encoding mismatch)
   - Idempotency failure (double-charge, duplicate event, re-entry)
   - Retry amplification (exponential backoff not respected, infinite retry loop)
   - Edge date or number (off-by-one, timezone, overflow, precision loss)
   - Authorization bypass (missing check, permission elevation)
   - Resource leak (connection pool exhaustion, file descriptor leak, goroutine leak)
   - Configuration drift (env mismatch, missing default, stale config)
3. For each bug class, ask: "Is the current implementation vulnerable to the same class of bug?"
4. Produce a table:

| Bug class | Source task | Current vulnerability | Test to write |
|-----------|-------------|----------------------|---------------|

### Pass beta: Seam Walk

Walk a fixed checklist of common-painful seams against the implementation. For each seam, assess vulnerability and design a test.

| Seam | What to check | Vulnerability (yes or no or dont know) | Test to write |
|------|---------------|----------------------------------------|---------------|
| **Concurrency** | Shared mutable state, goroutine races, mutex hot spots, channel deadlocks | | |
| **Retries** | Retry-able operations, backoff strategy, max-retries, jitter, circuit breaker | | |
| **Partial failures** | What breaks when one of N dependencies fails? CRDT or compensation? | | |
| **Idempotency** | Double-apply of same event or command, dedup key stability, exactly-once vs at-least-once | | |
| **Timeouts** | Deadline propagation, context cancellation, default timeout values, client-server timeout mismatch | | |
| **Data corruption** | Encoding or decoding mismatches, stale reads after write, concurrent write-write conflict, serialization version drift | | |
| **Edge dates and numbers** | Timezone handling, DST transitions, unix epoch boundaries, float precision, overflow on accumulation | | |
| **Hostile inputs** | SQL injection, path traversal, oversized payloads, unicode normalization, content-type confusion | | |

### Pass alpha: Free-form Red Team

Put on the adversary hat. "You want to break this implementation."

1. List 5-10 hypotheses about what could go wrong. Aim for specific, not generic: "If the event bus delivers a subscription.create event but the consumer receives it twice before the dedup window expires, the second receive will double-charge" — not "retries could fail."
2. For each hypothesis, design the falsifying test.
3. Mark each hypothesis as:
   - **Planned** — you designed the test, no evidence it is broken
   - **Confirmed-broken** — you found evidence in the code that this case would fail (reading the implementation or prior bugfix history)
   - **Dismissed-with-reason** — you investigated and ruled it out (document why)

| Hypothesis | Falsifying test | Status |
|------------|-----------------|--------|

## Phase 4: Write the Test Plan

Write the test plan as a single flicknote with two sections (Constructive and Adversarial).

### Template

```markdown
# Test Plan: <feature or component>

**Project:** <ttal alias>
**Implementation under test:** <files and relevant task UUIDs>
**Adversarial findings filed separately:** <flicknote hex if applicable, or "None">

## Constructive
### Happy paths
- ...
### Edge cases
- ...
### Invariants and preconditions
- ...

## Adversarial
### Pass gamma — Prior-bug classes
| Bug class | Source task | Current vulnerability | Test to write |
|-----------|-------------|----------------------|---------------|

### Pass beta — Seam walk
| Seam | Vulnerability | Test to write |
|------|---------------|---------------|

### Pass alpha — Red team hypotheses
| Hypothesis | Falsifying test | Status |
|------------|-----------------|--------|
```

### Storage

```bash
# Primary: test plan flicknote
cat <<'PLANEOF' | flicknote add --project testplans
# Test Plan: ...
...
PLANEOF

# Annotate the parent task with the hex ID
task <parent-uuid> annotate "testplan: flicknote <hex>"
```

### Secondary flicknote (conditional)

If pass gamma or beta found **confirmed-broken** (not just "could break"), write a separate bug or test report flicknote:

```bash
cat <<'BUGEOF' | flicknote add --project testplans
# Bug or Test Report: <feature or component>

**Confirmed-broken issues found during adversarial pass.**
**Source:** Test plan flicknote <hex>

## Issue 1: <title>
**Pass:** gamma or beta (which pass found it)
**Evidence:** <what in the code proves this would fail>
**Recommended test:** <test that would catch this>
...
BUGEOF

task <parent-uuid> annotate "bugreport: flicknote <hex>"
```

Do NOT auto-file +bugfix tasks. The skill writes evidence; humans decide whether to file.

## Phase 5: Validate

Before declaring the test plan done:

- [ ] Constructive section covers happy paths for all major flows
- [ ] Edge cases documented per entry point
- [ ] Invariants and preconditions documented
- [ ] Adversarial pass gamma ran (prior bugs collected, vulnerability table complete)
- [ ] Adversarial pass beta ran (all 8 seams checked, each row has a test or a vulnerability)
- [ ] Adversarial pass alpha ran (5-10 hypotheses, each with a falsifying test)
- [ ] Test plan flicknote written and annotated on parent task
- [ ] If confirmed-broken found: separate bug or test report flicknote written and annotated

## Testing and Iteration

No synthetic subagent tests for this skill. Per the team brainstorm verdict, the FSE sub plus neuron consume first-use IS the test. Friction observed during the first invocation feeds skill revision via flicknote-edit on this skill itself.

## After the Test Plan Is Written

Chain into the completion phase for self-review, open questions, summary, and review handoff:

    skill get sp-complete-design

Follow every step in that skill. Do not duplicate its logic here.

## Remember

- Constructive without adversarial is half a job — run all three passes
- Prior bugs are the empirical anchor — start with pass gamma
- Seam checklist (beta) is fixed — do not skip rows or paraphrase
- Red-team hat (alpha) needs 5-10 specific hypotheses — generic fears do not count
- Secondary flicknote for confirmed-broken only — not for "could break"
- Do NOT file +bugfix tasks — write evidence, humans triage
