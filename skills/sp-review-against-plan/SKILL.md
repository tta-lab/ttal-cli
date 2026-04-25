---
name: sp-review-against-plan
description: Review methodology — measure a PR against the plan that produced it; in-scope undone is always blocking
category: methodology
---

# Review Against Plan

## Overview

You wrote a plan. A worker executed it. Now you're reviewing their PR against that plan. Your job isn't to re-review the whole codebase — specialized agents (pr-review-lead etc.) do that. Your job is narrower and sharper: **did the PR actually deliver what the plan said it would?**

This skill exists because lenient "it's mostly done, minor gaps can be follow-up nits" reviews leak scope and waste pipeline cycles. Read the rule, then the anti-pattern, then do the review.

## The Rule

Every finding falls into exactly one of three categories:

1. **In-scope + done** ✓ — plan called for it, PR delivered it. No action. Don't bother listing these except as a sanity check.

2. **In-scope + undone** 🔴 — plan called for it, PR didn't deliver it (or delivered a partial/watered-down version). **ALWAYS BLOCKING.** Never "non-blocking." Never "fold into follow-up PR." Never "minor gap." If it was in the plan, it's in this PR — or the PR is not done.

3. **Cosmetic + no value** ⚪ — not in the plan, no real value if added. **Don't mention at all.** Not even as a "minor nit." Noise wastes attention.

There is no fourth category. No "minor gap, non-blocking, fold into follow-up" escape hatch. That hatch is how scope leaks.

## Non-criteria (don't flag these)

- **Commit count / commit message style** — we squash-merge. Worker's commit granularity is cosmetic and disappears at merge time. 3 commits vs 1 vs 5 — doesn't matter.
- **Formatting / whitespace** — covered by linters and pre-commit hooks.
- **Code style choices equivalent to what you'd have written** — if it works and matches the plan, don't re-grade against the version in your head.

## Anti-pattern (real example)

From an actual past review:

> Review complete. Build + tests + vet all pass. CI still running.
> Matches plan exactly:
> ...
> Minor gaps (non-blocking):
> 1. Aliyun 3-function refactor skipped — subtask e1f0a134 annotation asked for extracting three helper functions; worker inlined them instead.
> 2. One AssemblyAI edge case uncovered — we're still billed. Rare case. Could be folded into 7a8ee4f1 cleanup or a follow-up nit commit.

Both items are Category 2 (in-scope + undone). Both were in the plan's subtask annotations. Both got mislabeled as "non-blocking."

What went wrong:
- The reviewer felt the "spirit" of the plan was met, so forgave the specific asks.
- "Follow-up nit PR" sounds cheap but costs a full pipeline slot (plan → implement → review → merge) for work that was already scoped.
- The edge case that costs real money ("still billed") got called "rare" to defer it.

Correct classification:
- Aliyun refactor → Category 2 → BLOCKING → NEED_WORK.
- AssemblyAI edge case → Category 2 → BLOCKING → NEED_WORK.

## How to run the review

### 1. Load the plan

```bash
# Execution steps (what the worker was supposed to do):
task <task-uuid> tree

# Orientation (what/why/anti-goals — if present):
# Look for "orientation: flicknote <hex>" in the task annotations, then:
flicknote detail <hex>
```

### 2. Walk the plan, bucket each item

For each subtask in the tree and each explicit ask in annotations:

- Read the subtask body + annotations — understand what was actually asked for, including nuances buried in annotation text.
- Check the PR diff for evidence the ask was delivered (`git diff origin/main..HEAD` or the PR URL).
- Classify: Category 1 ✓ / Category 2 🔴 / Category 3 ⚪ (silent).

Do NOT grade ambition (would a different plan have been better?). You're grading execution against YOUR plan.

### 3. Emit the verdict

**LGTM** — zero Category 2 findings. Every plan item is delivered. Advance the pipeline:

```bash
ttal go <hex>
# spawns pr-review-lead for the deeper review pass
```

**NEED_WORK** — one or more Category 2 findings. Send blockers directly to the worker via heredoc (typically long, often includes code):

```bash
cat <<EOF | ttal send --to <hex>:coder
# NEED_WORK

## Blockers (Category 2 — in-scope, undone)

1. <concrete finding>
   Plan asked for: <quote/paraphrase from subtask or annotation>
   PR delivered:  <what's there, or nothing>
   Fix:           <what needs to happen>

2. <next finding>
   ...

## Questions (optional — ask before fixing if unclear)

- <question>
EOF
```

The worker fixes, pushes new commits, and you re-review. No `ttal comment` — `ttal send` is the single channel.

## Framing discipline

- **Findings are concrete.** "Plan said extract three helpers; PR inlined them." Not "code quality could be better."
- **Cite the plan.** Quote the annotation or reference the subtask UUID. The worker shouldn't have to guess what you're referring to.
- **Be terse.** Category 2 findings are mechanical: plan said X, PR has Y, fix Z. No essays.
- **Don't mix categories.** If it's Category 3, drop it. Mixing cosmetic wishes into blocker feedback dilutes the signal.

## When to escalate

- If the plan itself turns out to be wrong (worker's interpretation was reasonable, yours was unclear) → that's a plan-quality issue on you, not a NEED_WORK on the worker. Revise the plan, annotate the task, re-review.
- If the worker pushed back in a previous round and you now agree → retract that finding, don't double down.
- If the worker genuinely can't do the Category 2 item for reasons outside their control → escalate to Neil via `ttal send --to neil`. Don't downgrade to non-blocking to paper over it.
