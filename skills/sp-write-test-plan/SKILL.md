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
4. **Disambiguate scope when the task description references multiple feature or table names.** Wording like "sub command + neuron consume" or "purchases + consumptions" can map to different code surfaces (e.g., `neuron_purchases` writes vs `neuron_consumptions` writes — different services). If the description is ambiguous, ask the requester to confirm the exact scope BEFORE Phase 1. Ambiguous scope confirmed late costs more than confirmed early.

**Hard rule:** Do NOT proceed past this gate without a confirmed single target repo AND an unambiguous feature scope.

If the task touches multiple repos, stop and flag it: a single test plan must cover a single project. Split the task into per-repo test plans.

## Phase 1: Explore Reality

BEFORE writing any test plan, understand what exists. Do not design tests in the abstract — read the implementation.

1. **Read implementation files** — find and read the files and modules under test. Understand data flow, entry points, error paths, dependencies.
2. **Read related task tree** — `task {uuid} tree` — understand what steps the implementation went through, what decisions were made.
3. **Read orientation flicknotes** — `flicknote find <keywords>` — search for orientation docs, design docs, research notes related to this domain. **For non-obvious choices visible in the implementation, search for Q-numbered decisions** (e.g., "Q1 RESOLVED", "Q3 decision" — orientation flicknotes often record locked design choices this way). The Q-numbered entry is where the rationale lives; without it, code can look arbitrary.
4. **Read prior bugfix history** — `task +bugfix project:<alias> status:completed export` — extract descriptions of every past bug in this project.
5. **Diff merged code against recent design plans.** If you found a design plan flicknote referencing a PR that's now merged, run `git log --oneline path/to/changed/files` and read the actual current code — divergences from the plan (stricter guards, reordered operations, additional defensive checks) are regression-trap candidates. Surface them in pass γ. The plan describes intent; the merged code is reality.
6. **Map test surface** — what are the integration boundaries? External services, databases, file systems, network calls, async queues. List them.

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
| Seam | What to check | Vulnerability | Test to write |
|------|---------------|---------------|---------------|

### Pass alpha — Red team hypotheses
| Hypothesis | Falsifying test | Status |
|------------|-----------------|--------|

## Friction notes
Things the skill could have surfaced earlier, places the methodology felt heavy or thin, gaps in the storage template. Concrete enough to land as edits to `skills/sp-write-test-plan/SKILL.md` in a follow-up.
```

### Storage

```
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

```
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
- [ ] Friction notes captured in the test plan (skill-level gaps surfaced this invocation, concrete enough to land as edits to this skill)

## After the Test Plan Is Written

Chain into the completion phase for self-review, open questions, summary, and review handoff:

    skill get sp-complete-design

Follow every step in that skill. Do not duplicate its logic here.

## Phase 6: Execute (optional — when authoring meets running)

The skill's primary deliverable is the test plan. But sometimes the planner is also asked to **run** the plan, not just hand it off — pairing with a human, smoking findings, or running the adversarial passes against live infrastructure. When that happens, lessons from the first real-world execution session (eve+Neil 2026-05-06, fse.sub) apply:

### Forensic harvest before live tests

Read existing production state (DB rows, logs, audit trails) BEFORE running new tests. Many "NOT-RUN" rows can be promoted to PASS using evidence that already exists. Example: 12 cases moved to PASS in one DB sweep (event-type counts, key-set verification, HWM monotonicity check, signed_payload presence) without firing a single curl. The "running" wasn't writing tests; it was forensic-reading the production audit trail and matching observed behavior to test plan expectations.

**Practical pattern:** before each adversarial pass, query the relevant tables. Look for:
- Event-type distribution (which paths are exercised; where coverage gaps live in production traffic)
- Recent state transitions (do invariants hold across observed sequences?)
- Constraint outputs (UNIQUE-key dedup, NOT NULL enforcement, generated columns)
- Key-shape consistency (camelCase vs snake_case, populated vs NULL)

### Speculative-PASS pitfall

When a test scenario produces an observation that could be explained by multiple protective layers, **you have not verified the layer you intended**. Mark NOT-RUN with a caveat, not PASS.

Example failure mode (eve 16:43→16:55 on fse.sub): replayed an old VALIDATE while user was already expired; observed entitlement state preserved. Marked PASS for "cael's defensive guard works." Neil challenged "when did this PASS?" — re-examination showed the SQL HWM guard alone would have rejected the upsert independently of cael's guard, AND the user being already-expired meant the "doesn't downgrade an active user" assertion couldn't be tested. Retracted to NOT-RUN with a plan to retest properly post-resubscribe.

**Practical pattern:** before marking PASS, ask "which layer am I claiming this verifies, and could another layer also have produced this observation?" If yes, isolate before claiming.

### Pod-log smoking-gun pattern

When multiple protective layers chain (e.g., a defensive guard + an SQL HWM check), the response code alone won't distinguish which layer fired. Use slog evidence in pod logs as the tie-breaker.

Example: cael's `transactionExpired` guard logs `INFO skip entitlement upsert: transaction already expired ... handler=handleValidateSubscription` when it fires. Grep `kubectl logs` after the test to confirm that exact line appeared, not just "the response was 200." This is the proof that the specific layer you wanted to verify was the one doing the work.

### Pod-restart vs merge gap

`:latest` tag doesn't auto-rollout in Kubernetes. A merged PR's image won't reach the running pod until the deployment is restarted (`kubectl rollout restart deployment/{name}` or equivalent). Smoking against the wrong build looks like the fix worked when it didn't (or looks like the bug is still there when it isn't).

**Practical pattern:** before smoking a fix, verify pod age vs PR merge time. If `kubectl get pod -o jsonpath='{.metadata.creationTimestamp}'` predates the merge commit, the new image isn't running. Surface to whoever owns the deploy step (the Merge ≠ Deploy rule).

### FAIL vs NOT-RUN distinction

Confirmed-broken via code-read or analysis = **FAIL**, not NOT-RUN. NOT-RUN means "haven't exercised yet, don't know the outcome." FAIL means "outcome confirmed to diverge from expected, regardless of whether a test program ran."

Example: α12/α13 (cross-tenant injection) were originally NOT-RUN with notes about pending cert-chain analysis. After researcher verified no cryptographic defense exists and code-walk confirmed no app-side check, status flipped to FAIL — the bug is real even though no integration test exists yet. The fix-PR's unit tests are the regression coverage; integration smoke is positive-control only.

### Negative-control achievability gates

Some adversarial cases can't have a direct integration smoke (e.g., env-mismatch JWS requires Apple's private signing key for a different environment, which no team has). Recognize this upfront in the plan:
- Smoke covers POSITIVE control (legitimate paths still work post-fix)
- NEGATIVE control (rejection on bad input) lives in unit tests in the fix PR
- Don't chase a smoke gap that's not achievable

**Practical pattern:** when the fix PR ships with unit tests covering the negative path, accept those as the regression coverage. Live integration smoke is regression-on-positive-control, not exhaustive proof.

### Industry-wide unachievable scope

Some test-coverage goals are unachievable in any environment, by anyone. Recognize and document, don't chase. Example (fse.sub Apple webhook variants): RESCIND_CONSENT, METADATA_UPDATE, MIGRATE, RENEWAL_EXTENDED, EXTERNAL_PURCHASE_TOKEN, OFFER_REDEEMED, DID_FAIL_TO_RENEW, REFUND have no sandbox trigger mechanism per Apple. The industry-standard ceiling is fixture-replay with captured production JWS — which requires production traffic to accumulate. Pre-prod-deploy, this is unreachable.

**Practical pattern:** for plans covering Apple/Stripe/payment-processor webhooks (or similar third-party-controlled sources), add a "Realistic Completion Model" section noting which cases are sandbox-testable, which need fixture-corpus, and which have no path. Sets expectations for the next planner.

### Discover → file → fix → smoke loop in single session

Integration plans can produce immediate-value bugs, not just future test code. When the adversarial pass surfaces confirmed-broken behavior, the file-fix-deploy loop can close before the session ends. Example session (eve+Neil 2026-05-06): 4 +bugfix tasks filed, all 4 PR-merged + deployed within ~5 hours.

**Practical pattern:** when running adversarial passes live, expect to surface real bugs. Have the +bugfix-filing path warmed up (annotation template ready). Coordinate with manager/fixer agents (yuki, lux, kestrel) for the file-design-implement chain.

### Cross-agent collaboration during execution

Researchers (athena/quill) and fixers (lux/cael/kestrel) have specialized depth worth tapping. The skill's adversarial pass can be reinforced by an external researcher's review (different mental model surfaces different gaps). Plan-review handoffs to fixers during fix design close the loop on quality of recommendations.

**Practical pattern:** when scope warrants, loop in a researcher for adversarial review of the plan, and a fixer for plan-review of any +bugfix designs that come out of the plan. Document their contributions in the test plan flicknote (cross-reference researcher review notes, fixer plan-review threads).

### Living test report artifact

If running a paired session with the human, consider a JSONL-driven HTML report as a living artifact. Each curl converts a row from NOT-RUN to PASS in real-time; pod-log evidence pastes into the actual column. Different from a write-once test plan flicknote — this is execution-phase tracking with progress visualization.

**Practical pattern:** template at `templates/ttal/{agent}/test-report-<project>-<date>.html` (per-agent workspace, single self-contained file with embedded JSONL data + JS rendering). Don't conflate with the test plan flicknote — the flicknote is the methodology output, the HTML is the execution log.

## Remember

- Constructive without adversarial is half a job — run all three passes
- Prior bugs are the empirical anchor — start with pass gamma
- Seam checklist (beta) is fixed — do not skip rows or paraphrase
- Red-team hat (alpha) needs 5-10 specific hypotheses — generic fears do not count
- Secondary flicknote for confirmed-broken only — not for "could break"
- Do NOT file +bugfix tasks — write evidence, humans triage
- When executing the plan: forensic-harvest first, isolate-before-PASS, smoking-gun-via-logs, watch the merge≠deploy gap
