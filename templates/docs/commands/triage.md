---
name: triage
description: "Triage PR review comments — assess, fix what's actionable, post status update"
argument-hint: "[review-file-path]"
claude-code:
  allowed-tools:
    - Bash
    - Glob
    - Grep
    - Read
    - Edit
    - Write
    - Task

# Triage

Triage PR review comments: assess each one, fix what's actionable, then post a status update.

## Phase 1: Assess

### Input

If the reviewer notification includes a file path (e.g., `/tmp/ttal-review-XXXXX.md`),
read it first — it contains the full review. Skip `ttal pr comment list` for this review
round since you already have the content.

If no file path was provided, fall back to `ttal pr comment list`.

Read all review comments and categorize them.

### Evaluation Framework

For each comment, ask:

1. **Is it valid?** — Technically correct? Or misunderstanding codebase context/patterns?
2. **Is it worth fixing?** — Impact on correctness, security, maintainability?
3. **What's the effort?** — Low/medium effort = do it. High effort + low impact = defer.

**Don't dismiss issues because they're preexisting.** "Out of scope" and "predates this PR" are not reasons to defer. Evaluate by worth + effort, not by origin.

### Categories

**Actionable (fix now)**
- Technically valid and important
- Real bugs, security issues, significant problems
- Low effort regardless of impact (easy wins)
- Low-to-medium effort with medium-to-high value
- Cosmetic fixes: typos, naming, formatting — if low effort, just do them
- DRY violations: duplicated code/logic that should be extracted

Format: `[FIX] <summary> — <why it matters>`

**False Positive (push back)**
- Based on incorrect assumptions
- Misunderstand codebase context or patterns
- Would break existing behavior
- Apply rules that don't fit this codebase

Format: `[FALSE POSITIVE] <summary> — <why it's wrong, suggested response>`

**Deferrable (follow-up)**
- High effort but low impact
- Style preferences, not correctness
- Technically correct but over-engineered for current needs

Format: `[DEFER] <summary> — <why it can wait>`

### Proactive Checks

Beyond reviewer comments, scan the PR diff for:

- **DRY violations** — duplicated logic, copy-pasted blocks, patterns that should be shared
- **Cosmetic issues** — typos, misleading names, inconsistent formatting

Include these as actionable even if the reviewer didn't mention them.

## Phase 2: Fix

Address all actionable items. Then move to Phase 3.

## Phase 3: Report

Post a status update to the PR with what's done, what's remaining, and what you're pushing back on.

### Gather Evidence

```bash
# View PR comments
ttal pr comment list

# Check what's changed since review
git log --oneline origin/main..HEAD
git diff origin/main..HEAD
```

For each item, verify against code: search for implementations, check tests, confirm fixes.

### Post Update

**After fixing NEEDS_WORK items** — post **without** `--no-review` to trigger re-review:

```bash
ttal pr comment create "<markdown>"
```

This tells the reviewer "I've addressed your feedback, please look again."

**After LGTM with no remaining issues** — use `--no-review` to avoid a pointless review loop:

```bash
ttal pr comment create --no-review "<markdown>"
```

**Rule of thumb:** If you fixed things and want confirmation, omit `--no-review`. Only use `--no-review` when the review is already passing and you're posting a status note before merging.

Format:

```markdown
## Triage Update

### Fixed
- [x] Item — *addressed in commit abc123*
- [x] Item — *implemented in `src/file.ts:42`*

### False Positive
- [ ] Item — *[reason, suggested response]*

### Deferred
- [ ] Item — *[reason, follow-up plan]*

### Remaining
- [ ] Item — *[what's still needed]*
```

## Guidelines

- Be objective — don't dismiss valid criticism
- Provide concrete reasoning for each categorization
- Provide evidence for fixed items (commit SHA, file:line)
- For false positives, explain why and suggest a response
- For deferrals, be specific about why it can wait
- If comments reference code you haven't seen, read it first
- Cosmetic and DRY fixes are easy wins — don't defer them, just do them
- Evaluate by worth + effort, never by scope or origin
