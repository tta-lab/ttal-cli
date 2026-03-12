---
name: sp-writing-plans
description: Use when you have a spec or requirements for a multi-step task, before touching code
---

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the worker has zero context for our codebase. Document everything they need: which files to touch, before/after code, build/test commands, commit messages. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Assume the worker is a skilled developer, but knows almost nothing about our toolset or problem domain.

**Announce at start:** "I'm using the writing-plans skill to create the implementation plan."

## Designer Rules

1. **Never write code** — you write plans, not implementations
2. **Never execute without approval** — plans wait for explicit go-ahead
3. **Always plan first** — no exceptions, even for "quick fixes"
4. **Design at structure level** — before adding behavior, question whether the existing structure supports it cleanly. Refactor first if needed.

## Design Discipline

- **Look for abstractions before patching:** When fixing a bug, ask "what are the right primitives?" not just "how do I fix this case?"
- **Treat justified duplication as a smell:** If you catch yourself saying "this duplication is fine because X is rare," that's a signal to refactor, not rationalize
- **Design at structure level, not code level:** Before adding new behavior, question whether the existing structure supports it cleanly. Refactor first if needed.

## Plan Quality Checklist

Every task in the plan MUST have:

- [ ] **Files** — exact paths to create, modify, and test
- [ ] **Before/after code** — show what changes, not just "add validation"
- [ ] **Build/test commands** — exact commands with expected output
- [ ] **Commit message** — ready to copy-paste
- [ ] **Dependencies explicit** — what must be done before this task
- [ ] **Self-contained** — worker can execute without asking questions

If a task fails this checklist, it's not ready.

## Plan Document Header

Every plan MUST start with:

```markdown
# Plan: [Feature Name]

> **For Claude:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task.

**Goal:** [One sentence describing what this builds]

**Depends on:** [Other plans/tasks, or "None"]

---
```

## Task Structure

```markdown
## Task N: [Component Name]

**Files:**
- Create: `exact/path/to/file.go`
- Modify: `exact/path/to/existing.go`
- Test: `tests/exact/path/to/test.go`

**Step 1: Write the failing test**

\`\`\`go
func TestSpecificBehavior(t *testing.T) {
    result := Function(input)
    assert.Equal(t, expected, result)
}
\`\`\`

**Step 2: Run test to verify it fails**

Run: `go test ./path/... -run TestSpecificBehavior -v`
Expected: FAIL

**Step 3: Write minimal implementation**

\`\`\`go
func Function(input string) string {
    return expected
}
\`\`\`

**Step 4: Run test to verify it passes**

Run: `go test ./path/... -run TestSpecificBehavior -v`
Expected: PASS

**Step 5: Commit**

\`\`\`
feat(scope): add specific feature
\`\`\`
```

## Bite-Sized Task Granularity

Each step is one action (2-5 minutes):
- "Write the failing test" — step
- "Run it to make sure it fails" — step
- "Implement the minimal code to make the test pass" — step
- "Run the tests and make sure they pass" — step
- "Commit" — step

## Inline Plans vs Flicknote Plans

**Small tasks (≤6 steps, single file or mechanical changes):** Use inline plans — annotate the task directly with the plan. No flicknote needed.

```bash
ttal task add --project <alias> "description" --tag planned
task <uuid> annotate 'Plan (inline): 1. Remove dep from package.json 2. Update imports in 3 files 3. Run bun install'
```

**Large tasks (architecture decisions, multi-file refactors, trade-off analysis needed):** Use flicknote plans — save to flicknote, annotate task with hex ID.

```bash
flicknote add 'full plan content' --project <your-project>
task <uuid> annotate 'Plan: flicknote <hex-id>'
```

**Decision rule:** If the plan fits in 1-2 task annotations and a worker can execute it without ambiguity, inline it. If it needs headings, code examples, trade-off analysis, or context sections — use flicknote.

## After the Plan Is Written

1. **Save the plan** — inline annotation or flicknote (see above)
2. **Create a task** (if needed): `ttal task add --project <alias> "description" --tag planned`
3. **Annotate the task** with plan reference (inline or flicknote hex ID)
4. **Tag as planned:** `task <uuid> modify +planned`
5. **Wait for approval** — never start execution yourself
6. **On approval:** `ttal task execute <uuid>` spawns a worker

## Remember

- Exact file paths always
- Complete code in plan (not "add validation")
- Exact commands with expected output
- DRY, YAGNI, TDD, frequent commits
- Worker should never need to ask questions
