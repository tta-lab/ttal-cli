---
name: sp-debugging
description: Diagnose bugs systematically and write bug fix plans for workers to execute — diagnosis + plan writing in one skill
---

# Bug Fix Design

## Overview

Diagnose bugs by tracing from symptom to root cause, then write a fix plan clear enough for a worker to execute without guessing. No fixes without diagnosis. No diagnosis without evidence.

**Core principle:** ALWAYS find root cause before writing the fix plan. Symptom fixes are failure.

**Announce at start:** "I'm using the debugging skill to diagnose this bug and write a fix plan."

## Designer Rules

1. **Never write code** — you write fix plans, not implementations
2. **Never execute without approval** — plans wait for explicit go-ahead
3. **Always diagnose first** — no exceptions, even for "obvious" bugs
4. **Design at structure level** — before adding behavior, question whether the existing structure supports it cleanly. Refactor first if needed.

## Phase 1: Root Cause Investigation

**BEFORE writing ANY fix plan:**

1. **Read Error Messages Carefully**
   - Don't skip past errors or warnings
   - They often contain the exact solution
   - Read stack traces completely
   - Note line numbers, file paths, error codes

2. **Reproduce Consistently**
   - Can you trigger it reliably?
   - What are the exact steps?
   - Does it happen every time?
   - If not reproducible → gather more data, don't guess

3. **Check Recent Changes**
   - What changed that could cause this?
   - Git diff, recent commits
   - New dependencies, config changes
   - Environmental differences

4. **Gather Evidence in Multi-Component Systems**

   **WHEN system has multiple components (CI → build → signing, API → service → database):**

   ```
   For EACH component boundary:
     - Log what data enters component
     - Log what data exits component
     - Verify environment/config propagation
     - Check state at each layer

   Run once to gather evidence showing WHERE it breaks
   THEN analyze evidence to identify failing component
   THEN investigate that specific component
   ```

5. **Trace Data Flow**

   See `root-cause-tracing.md` in this directory for the complete backward tracing technique.

   **Quick version:**
   - Where does bad value originate?
   - What called this with bad value?
   - Keep tracing up until you find the source
   - Fix at source, not at symptom

## Phase 2: Pattern Analysis

1. **Find Working Examples** — locate similar working code in the same codebase
2. **Compare Against References** — read reference implementations COMPLETELY, don't skim
3. **Identify Differences** — list every difference between working and broken, however small
4. **Understand Dependencies** — what components, settings, config, environment does this need?

## Phase 3: Hypothesis and Testing

1. **Form Single Hypothesis** — "I think X is the root cause because Y"
2. **Test Minimally** — smallest possible change to test hypothesis, one variable at a time
3. **Verify** — did it work? Yes → write fix plan. No → form NEW hypothesis. Don't stack fixes.
4. **If 3+ Hypotheses Failed** — STOP. Question the architecture. This is a wrong pattern, not a missing fix. Discuss with your human partner before continuing.

## Phase 4: Write the Fix Plan

Once root cause is confirmed, write the fix plan.

### Fix Plan Structure

```markdown
# Fix: [Bug Title]

> **For Claude:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task.

## Symptom
[What the user/system sees — error messages, unexpected behavior]

## Root Cause
[What's actually broken and why — the diagnosis, not the symptom]

## Reproduction
[Exact steps to trigger the bug]

## Fix Strategy
[What needs to change and why this approach over alternatives]

## Implementation Tasks

### Task 1: [Component Name]

**Files:**
- Modify: `exact/path/to/file.go`
- Test: `tests/exact/path/to/test.go`

**Step 1: Write the failing test**
[exact test code]

**Step 2: Run test to verify it fails**
Run: `go test ./path/... -run TestName -v`
Expected: FAIL

**Step 3: Write minimal fix**
[exact code change — before/after]

**Step 4: Run test to verify it passes**
Run: `go test ./path/... -run TestName -v`
Expected: PASS

**Step 5: Commit**
`fix(scope): description`

## Verification
[How the worker confirms the bug is actually fixed — beyond just tests passing]
```

### Plan Quality Checklist

Every task in the plan MUST have:

- [ ] **Files** — exact paths to create, modify, and test
- [ ] **Before/after code** — show what changes, not just "add validation"
- [ ] **Build/test commands** — exact commands with expected output
- [ ] **Commit message** — ready to copy-paste
- [ ] **Dependencies explicit** — what must be done before this task
- [ ] **Self-contained** — worker can execute without asking questions

If a task fails this checklist, it's not ready.

### Bite-Sized Task Granularity

Each step is one action (2-5 minutes):
- "Write the failing test" — step
- "Run it to make sure it fails" — step
- "Implement the minimal fix" — step
- "Run the tests and make sure they pass" — step
- "Commit" — step

## Design Discipline

- **Look for abstractions before patching:** When fixing a bug, ask "what are the right primitives?" not just "how do I fix this case?"
- **Treat justified duplication as a smell:** If you catch yourself saying "this duplication is fine because X is rare," that's a signal to refactor, not rationalize
- **Design at structure level, not code level:** Before adding new behavior, question whether the existing structure supports it cleanly. Refactor first if needed.

## Inline vs Flicknote Fix Plans

**Small fixes (≤6 steps, single file or mechanical changes):** Use inline plans — annotate the task directly. No flicknote needed.

```bash
task <uuid> annotate 'Fix (inline): Root cause: nil pointer in auth middleware. Fix: 1. Add nil check in middleware.go:42 2. Add test 3. Run tests'
```

**Large fixes (multi-file, needs diagnosis context, trade-off analysis):** Use flicknote — save full fix plan, annotate task with hex ID.

```bash
flicknote add 'full fix plan content' --project <your-project>
task <uuid> annotate 'Fix plan: flicknote <hex-id>'
```

**Decision rule:** If the plan fits in 1-2 task annotations and a worker can execute it without ambiguity, inline it. If it needs headings, code examples, trade-off analysis, or context sections — use flicknote.

## After the Fix Plan Is Written

1. **Save the plan** — inline annotation or flicknote (see above)
2. **Create a task** (if needed) via `ttal task add --project <alias> "description"`
3. **Annotate the task** with plan reference (inline or flicknote hex ID)
4. **Review:** Run at least 1 round of `ttal go <uuid>`. Revise if needed.
5. **Execute:** When the plan passes review, run `ttal go <uuid>` to spawn a worker.

## Red Flags — STOP and Return to Phase 1

If you catch yourself thinking:
- "Quick fix for now, investigate later"
- "Just try changing X and see if it works"
- "It's probably X, let me fix that"
- "I don't fully understand but this might work"
- "One more fix attempt" (when already tried 2+)
- Proposing solutions before tracing data flow
- Each fix reveals new problem in different place

**ALL of these mean: STOP. Go back to diagnosis.**

## Supporting Techniques

Available in this directory:

- **`root-cause-tracing.md`** — trace bugs backward through call stack to find original trigger
- **`defense-in-depth.md`** — add validation at multiple layers after finding root cause
- **`condition-based-waiting.md`** — replace arbitrary timeouts with condition polling

## Remember

- Root cause first, always — no plan without diagnosis
- Exact file paths, complete code, exact commands
- One bug = one plan = one task = one worker
- DRY, YAGNI, TDD, frequent commits
- Worker should never need to ask questions
