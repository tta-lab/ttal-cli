---
name: task-route
description: "Classify a task's readiness and route it to the right next step"
argument-hint: "<task-uuid or keyword>"
claude-code:
  allowed-tools:
    - Bash

# ttal-route

Inspect a task and decide where it goes next. Tasks move through five states before a worker ever touches them — don't skip ahead.

Both `ttal task route` and `ttal task execute` have built-in confirmation gates — run the command directly after deciding.

## Usage

```
/ttal-route <task-uuid>
```

## Workflow

1. Resolve the UUID if given a keyword:
   ```bash
   ttal task find <keyword>
   ```

2. Read the task:
   ```bash
   ttal task get <uuid>
   ```

3. List available agents:
   ```bash
   ttal agent list
   ```

4. Apply the decision tree (in order — stop at the first match):

   ```
   ask → brainstorm → investigate (research/fix/design) → plan → execute
   ```

5. **Present your recommendation** — explain in one sentence which state you picked and what command you'd run. Include agent emojis from `ttal agent list` to make routing options scannable. Then run it.

## Decision Tree

Work through these in order. Stop at the first match.

### 1. Ask — task is too vague to do anything with
The description is ambiguous, missing key information, or could mean multiple different things. A worker or agent would have to guess.

**Recommend:** Ask Neil a clarifying question. Tag and annotate the task:
```bash
task <uuid> modify +ask
task <uuid> annotate "blocked: waiting for clarification — <your question>"
```

### 2. Brainstorm — task has direction but no design yet
The goal is clear enough to work with, but how to get there hasn't been explored. There's no plan, no research, and no obvious implementation path.

**Recommend:** Route to a design/brainstorm agent, instructing them to brainstorm first.
```bash
ttal task route <uuid> --to <agent-name> --message "please use /sp-brainstorming to explore this"
```

### 3. Investigate — facts are missing, but what kind?

Three kinds of investigation — route to the right agent based on where the unknowns live:

**3a. Research (external)** — unknowns are outside our codebase. New APIs, unfamiliar libraries, third-party repos, web research, comparisons.

**Recommend:** Route to a research agent.
```bash
ttal task route <uuid> --to <agent-name>
```

**3b. Fix/Diagnose (internal)** — unknowns are bugs or issues in our own code. Tracing our own pipelines, diagnosing errors, reproducing failures.

**Recommend:** Route to a fixer agent.
```bash
ttal task route <uuid> --to <agent-name>
```

**3c. Design (internal architecture)** — unknowns are about structure in our own code. Refactors, module boundaries, dependency untangling.

**Recommend:** Route to a designer agent.
```bash
ttal task route <uuid> --to <agent-name>
```

### 4. Design — problem and facts are known, solution needs a plan
Good understanding of the problem and constraints, but no implementation plan yet. A worker would need a structured approach before coding.

**Recommend:** Route to a design agent.
```bash
ttal task route <uuid> --to <agent-name>
```

### 5. Execute — plan or design doc is annotated, ready to implement
Has a `plan:`, `flicknote/` plan, or design doc annotated. Research alone is NOT enough — research must lead to a design/plan before executing. A worker can pick this up and implement it.

**Recommend:**
```bash
ttal task execute <uuid>
```

## Using --message

`--message` adds task-specific context to the routing prompt. Use it when the task annotation alone doesn't give the receiving agent enough focus — for example, a specific constraint, a pointer to related code, or a scope limit.

```bash
ttal task route <uuid> --to <agent-name> --message "focus on the rate limiting behaviour in the auth module"
```

Don't use `--message` to tell agents which skill to use — that's already defined by their role in `roles.toml`.

## Annotations

While inspecting a task, if you discover new information — a related file, a constraint, a clarification, anything that would help the next agent — annotate it before routing:

```bash
task <uuid> annotate "found: auth module uses JWT, not sessions"
task <uuid> annotate "note: this is blocked by task abc12345"
```

Don't wait for the worker to discover it themselves. Annotations accumulate context that makes routing decisions better over time.

## Direction Change

Sometimes while inspecting a task, you realize the task itself is wrong — the direction changed, scope shifted, or it's been superseded. Don't force-route a stale task.

### Replace — task description no longer matches intent
The original task was written with assumptions that no longer hold. A new task with the correct scope is needed.

**Recommend:** Create a new task and delete the old one:
```bash
ttal task add --project <alias> "updated description" --annotate "replaces <old-uuid>"
```
Then use the **task-deleter** subagent to remove the old task.

### Remove — task is no longer needed
The task was completed by another task, made irrelevant by a design change, or duplicates existing work.

**Recommend:** Delete it using the **task-deleter** subagent. Explain why it's no longer needed.

### Rescope — task is partially right but needs adjustment
The core intent is still valid, but the description or annotations are outdated. Modify in place rather than replacing.

**Recommend:** Update the task directly:
```bash
task <uuid> modify "updated description"
task <uuid> annotate "rescoped: <what changed and why>"
```

**Present these recommendations the same way as routing decisions — explain what you'd do, then act.**

## Decision Guide

| Task state | Route |
|------------|-------|
| Too vague — can't tell what's wanted | Ask Neil (`+ask`) |
| Direction changed, description is wrong | Replace with new task, delete old |
| No longer needed or duplicated | Remove (task-deleter) |
| Core intent valid, details outdated | Rescope (modify in place) |
| Clear goal, no design yet | Brainstorm → design agent |
| Needs external facts (APIs, libraries, web) | Research agent (external investigation) |
| Bug or issue in our own code | Fixer agent (internal diagnosis) |
| Structure/refactor question in our own code | Designer agent (internal architecture) |
| Facts known, needs implementation plan | Design agent |
| Research/investigation annotated, no plan yet | Design agent (investigate → plan → execute) |
| Plan/design doc annotated | Execute |

Use `ttal agent list` — don't hardcode agent names, the team changes.

**Important:** Your decision label must match the command you run:
- **Execute** → `ttal task execute` (spawns a worker)
- **Research/Brainstorm/Design** → `ttal task route --to <agent>` (sends to an agent)

Don't say "Execute" then route to an agent, or vice versa.

## Recommendation Style

When presenting routing recommendations, be conversational and include agent emojis. Give Neil a brief take on the task — what's interesting, what the approach would be, and who should handle it. Offer alternatives when reasonable.

**Good:**
> Created 92ff814a — Remove project_path UDA — use projects.toml as SSOT, validate in hooks +refactor priority:H
>
> This is a nice cleanup — projects.toml becomes the single source of truth, hooks enforce validity with good error messages. Route to Inke for design, or straight to Kestrel? 🐙🦅

**Bad:**
> Task 92ff814a needs design work. Recommend routing to inke.
> Waiting for your go.

Use emojis from `ttal agent list` when mentioning agents by name.

## Examples

```
/ttal-route b2c491aa
```
> "improve the thing" — too vague to act on. What does "improve" mean here? Performance? UX? Code quality? 🤔
>
> **Recommend:** Tag `+ask` and get clarity before routing.

---

```
/ttal-route a3f920c1
```
> Clear goal, no existing design. This needs brainstorming before anyone writes a plan.
>
> **Recommend:** Route to 🦉 Athena for brainstorming → `ttal task route a3f920c1 --to athena`

---

```
/ttal-route e18379f3
```
> Flicknote plan annotated, reviewed and approved — ready to ship. 🚀
>
> **Recommend:** `ttal task execute e18379f3`
