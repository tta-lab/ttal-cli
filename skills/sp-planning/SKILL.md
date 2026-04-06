---
name: sp-planning
description: Full planning process — explore reality, design, write plan, validate — before touching code
---

# Planning

## Overview

Plan by understanding reality first, then designing, then writing. Bad plans come from designers who don't read the codebase first.

Assume the worker is a skilled developer, but knows almost nothing about our toolset or problem domain. Document everything they need: which files to touch, a clear description of what to do (before/after code for non-obvious changes), build/test commands, commit messages. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

**Announce at start:** "I'm using the planning skill to create the implementation plan."

**First action:** Run `ttal project list` to identify the target project before writing anything.

## Designer Rules

1. **Never write code** — you write plans, not implementations
2. **Never execute without approval** — plans wait for explicit go-ahead
3. **Always plan first** — no exceptions, even for "quick fixes"
4. **Design at structure level** — before adding behavior, question whether the existing structure supports it cleanly. Refactor first if needed.

## Project Scope Gate

**Rule: 1 task → 1 plan → 1 project/repo.**

Before writing any plan, you MUST confirm the target project:

1. **Run `ttal project list`** — see all available projects
2. **Check the task's project field** — if it has one, use it as a hint for the target alias, then confirm with `ttal project get <alias>`
3. **If no project field** — ask explicitly: "Which repo does this plan target?"
4. **Validate the repo exists** — run `ttal project get <alias>` to confirm the path

**Hard rule:** Do NOT proceed past this gate without a confirmed single target repo.

### When Scope Is Too Big

If a task touches multiple projects/repos, the scope is too big for a single plan. Split it:

1. **Identify the repos involved** — list every repo the task would touch
2. **Extract separate tasks** — one task per repo, each with its own clear goal
3. **Set task dependencies** — use `task <uuid> modify depends:<other-uuid>` so phases execute in order (`ttal` doesn't expose `depends` — use taskwarrior directly for this)
4. **Write separate plans** — one plan per task, each scoped to its single repo
5. **Link plans via `Depends on`** — reference the other plan/task in the header

Example: "Add auth to API and update frontend to use it" → split into:
- Task A: `ttal task add --project api "Add auth endpoint"` → Plan A (api repo)
- Task B: `ttal task add --project frontend "Integrate auth endpoint"` → Plan B (frontend repo), depends on Task A

### Scope Violations — What to Flag

- Plan references files in multiple repos → **stop and split** into separate tasks + plans
- Task touches multiple projects → scope is too big, extract per-repo tasks with dependencies
- Plan says "update the API and the frontend" without specifying they're the same repo → clarify, likely needs splitting
- Task has no project field and plan doesn't state one → ask before writing
- Plan's target repo doesn't match the task's project field → reconcile before proceeding

## Phase 1: Explore Reality

**BEFORE writing ANY plan, understand what exists.** Don't design in the abstract — read the code.

1. **Read existing code** — find and read the files/modules the task would touch. Understand current patterns, naming conventions, architecture.
2. **Identify constraints** — what does the existing code assume? What interfaces, types, configs are already in place?
3. **Check prior art** — has this been attempted before? Search flicknote (`flicknote find <keywords>`) and completed tasks (`ttal task find <keywords> --completed`) for related work.
4. **Map dependencies** — what must exist before this works? What other modules, services, configs does this touch?

**Output:** A mental model of the current state. If something surprises you, note it — surprises in Phase 1 prevent disasters in implementation.

### Red Flags — STOP and Investigate

- You're planning changes to code you haven't read → read it first
- The task assumes a structure that doesn't exist → flag it
- You find recent changes that conflict with the task's assumptions → reconcile before planning

## Checkpoint 1: Discuss Approach

After Phase 1, talk through what you found before designing. Don't go silent and start writing — discuss first.

**Conversational checkpoint:**
- State what you found in the codebase: current patterns, constraints, surprises from Phase 1 (conversational summary, not written output)
- Propose your approach: 'I'm thinking we should do X because Y. The trade-off is Z.'
- Ask for alignment: 'Does this approach make sense, or should I consider something different?'
- Keep it lightweight: 3-5 sentences of understanding + proposed approach + a question
- **If not aligned** → revise approach and discuss again. Do not proceed to Phase 2 without explicit agreement.

**⛔ STOP here.** End your message after presenting your findings and question. Do not begin Phase 2 or write the orientation doc until the human responds and confirms alignment.

**Then, for complex tasks, capture the orientation:**

```bash
cat <<'ORIENT' | flicknote add --project orientation
# Orientation: [Feature Name]
## What
One sentence: what are we building/changing?
## Why
What problem does this solve? What's the motivation?
## Approach
Which approach are we taking and why? (If you evaluated alternatives in Phase 1, note the decision here.)
## Anti-goals
What is this NOT doing? (Prevents scope creep during implementation.)
ORIENT
```

Annotate the task: `task <uuid> annotate "orientation: <flicknote-hex-id>"`

**When to skip the orientation doc:** Simple bug fixes, mechanical refactors, tasks where what/why is obvious from the description. If the task description already answers what/why/approach, skip the doc. The conversational checkpoint still happens.

**When to write the orientation doc:** Multi-file features, architecture decisions, anything where a worker might ask 'but why are we doing it this way?'

---

(Note: the orientation doc writing happens AFTER the conversation confirms the approach — not before.)

## Phase 2: Design

With reality understood, now design the solution:

1. **Define exit criteria** — what does "done" look like? How does the worker know they've succeeded? Be specific: "all tests pass" is not enough — which tests, what behavior.
2. **Define anti-goals** — what is this plan NOT doing? Prevents scope creep during implementation.
3. **Evaluate scope** — can this be split into smaller PRs? Plans touching 40+ files are red flags. Prefer incremental delivery.
4. **Choose the approach** — given what you found in Phase 1, what's the simplest path? Consider alternatives briefly, pick one, justify it.
5. **Identify test strategy** — unit, integration, manual? Workers shouldn't have to decide this.

## Design Discipline

- **Look for abstractions before patching:** When fixing a bug, ask "what are the right primitives?" not just "how do I fix this case?"
- **Treat justified duplication as a smell:** If you catch yourself saying "this duplication is fine because X is rare," that's a signal to refactor, not rationalize
- **Design at structure level, not code level:** Before adding new behavior, question whether the existing structure supports it cleanly. Refactor first if needed.

## Phase 3: Write the Plan

Now write. The default format is a **task tree** — subtasks under the parent task. Each subtask = one step the worker executes and marks done. Use flicknote for orientation docs (what/why context) alongside the tree when needed.

### Plan Quality Checklist

Every subtask in the plan MUST have:

- [ ] **Files** — exact paths to create, modify, and test
- [ ] **What to do** — clear description, not just "add validation." For complex changes, include before/after code.
- [ ] **Build/test commands** — exact commands with expected output (can be in parent task or final subtask)
- [ ] **Commit message** — ready to copy-paste
- [ ] **Self-contained** — worker can execute without asking questions

If a subtask fails this checklist, it's not ready.

### Orientation Header (flicknote)

When writing a flicknote orientation doc alongside the task tree, start with:

```markdown
# Plan: [Feature Name]

**Project:** [ttal project alias — e.g. `ttal-cli`]
**Goal:** [One sentence describing what this builds]
**Anti-goals:** [What this plan is NOT doing — or "None"]
**Depends on:** [Other plans/tasks, or "None"]

---
```

### Task Structure (task tree format)

Each `##` heading becomes a subtask. Body text becomes the subtask's annotation. Workers see these via `task <uuid> tree` and mark each done on completion.

```markdown
## Add validation layer
Add input validation to the API handler. Check required fields, validate types, return 400 on failure.

Files: `internal/api/handler.go` (modify), `internal/api/handler_test.go` (create)

Step 1: Write failing test for missing required fields
Step 2: Implement validation, run tests
Step 3: Commit — `feat(api): add input validation to handler`

## Write integration tests
Integration tests for the full validation flow.

Files: `internal/api/integration_test.go` (create)

Step 1: Write integration test hitting the handler endpoint
Step 2: Run `make test`, verify pass
Step 3: Commit — `test(api): add validation integration tests`
```

Each subtask is self-contained: files, steps, commit message. The worker executes them in order and marks each done with `task <subtask-uuid> done`.

Subtasks execute in tree order — arrange them accordingly. For hard ordering constraints, use `task <uuid> modify depends:<other-uuid>`.

### Bite-Sized Task Granularity

Each step is one action (2-5 minutes):
- "Write the failing test" — step
- "Run it to make sure it fails" — step
- "Implement the minimal code to make the test pass" — step
- "Run the tests and make sure they pass" — step
- "Commit" — step

## Phase 4: Validate

Before declaring the plan done, check it against reality:

1. **Does the plan account for what you found in Phase 1?** — constraints, existing patterns, dependencies
2. **Are the file paths real?** — verify every path in the plan exists (or is clearly marked as "Create")
3. **Is the scope reasonable?** — if it's more than ~5 tasks, consider splitting into phases
4. **Would a worker need to ask questions?** — if yes, the plan isn't ready

## Plan Storage

### Inline Plans (small tasks)

For tasks with <=6 steps or single-file mechanical changes, annotate the task directly:

```bash
ttal task add --project <alias> "description" --tag planned
task <uuid> annotate 'Plan: 1. Do X 2. Do Y 3. Do Z'
```

### Task Tree Plans (default for structured plans)

For multi-step plans, create a subtask tree. The tree IS the plan — each subtask is a step, annotations hold details.

```bash
cat <<'PLAN' | task <parent-uuid> plan
## Step 1: Title
Details and context for this step.

## Step 2: Title
Details and context for this step.
PLAN

# View the plan
task <parent-uuid> tree

# Iterate
cat updated.md | task <parent-uuid> plan replace
```

No separate annotation needed — the subtasks are already under the parent task.

### Flicknote (orientation docs + legacy plans)

For orientation docs (what/why context): `flicknote add --project orientation`
For full plan docs (legacy, still supported): `flicknote add --project plans`

```bash
task <uuid> annotate 'orientation: flicknote <hex-id>'
# or for legacy plans:
task <uuid> annotate 'plan: flicknote <hex-id>'
```

**Decision rule:** If the plan fits in annotations → inline. If it's an ordered set of execution steps → task tree. If you need to capture what/why/trade-offs separately → flicknote orientation alongside a task tree.

## After the Plan Is Written

1. **Save the plan:**
   - Inline: annotate the task directly
   - Task tree: `cat plan.md | task <parent-uuid> plan` — subtasks are already under the parent
   - Flicknote (legacy): `flicknote add --project plans`, then `task <uuid> annotate '<hex-id>'`
2. **Review:** Run at least 2 rounds of `ttal go <uuid>`. Revise until the plan passes.
3. **Execute:** When the plan survives review, run `ttal go <uuid>` to spawn a worker.

## Remember

- Explore reality before designing — read the code first
- Exact file paths always
- Clear description of what to do; before/after code for non-obvious changes
- Exact commands with expected output
- DRY, YAGNI, TDD, frequent commits
- Worker should never need to ask questions
