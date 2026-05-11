---
name: sp-complete-design
description: "Completion phase for a written plan. Self-review, gather missing input, update state, and request review — with explicit output-channel partitioning so only summaries and open questions reach the human."
category: methodology
---

# Complete a Design Plan

## Overview

After a plan is written (by sp-planning or sp-debugging), this skill handles the final handoff phase: self-review, open questions, external research, state updates, and review routing.

Follow each step in order. Steps tagged **(→ persist)** write to state without surfacing to Neil. Steps tagged **(→ human)** output to Neil's context window.

---

## Step 1: Worker's-Eye Self-Review  (→ persist)

Re-read the plan as the worker who has to execute it. For every step ask:

- Could a worker execute this without asking?
- Are file paths, commands, and expected outputs exact?
- Are there hidden assumptions?

Note gaps to yourself only. Fix them in-place (edit the task tree / flicknote). No human-channel output from this step.

### Step 2: Identify Open Questions for Neil  (→ persist, then → human)

List decisions that genuinely need Neil's input (trade-offs you can't resolve, scope ambiguities, close architecture calls).

Write the list to persist first (annotate the task or append the orientation flicknote).

Then surface **only the questions** to the human channel — no reasoning, no alternatives, no self-review findings.

If there are no open questions, skip the human surface entirely.

### Step 3: Gather External Context If Needed  (→ persist)

If the plan needs cross-project reference, docs lookup, or web research, dispatch async:

```
ei ask --async "…" --project <alias>
ei ask --async "…" --web
```

Results land in `~/.einai/outputs/`. Fold findings back into the plan. Do not narrate the research to Neil.

### Step 4: Update the Plan with Review Findings and Answers  (→ persist)

Apply everything from steps 1–3 directly to the task tree / flicknote / annotations. Silent persist work:

```
task <uuid> plan
flicknote modify <id>
task <uuid> annotate '<note>'
```

### Step 5: Present the Summary  (→ human)

Surface only a concise summary of the finalized plan to Neil:

- One-paragraph what + why
- Bullet list of the major steps (not the details — those live in the tree)
- Any remaining risks or trade-offs

Keep it under ~200 words. No code blocks, no file paths, no step-by-step detail.

### Step 6: Request Review  (→ persist)

Run `ttal go <uuid>` to route the task to the plan reviewer. No human-channel output — the command itself is the handoff.

---

### Channel Discipline Checklist

- [ ] Self-review stayed in persist — no leakage to human channel
- [ ] Only open questions hit human in step 2 — no reasoning or alternatives surfaced
- [ ] Research dispatched async and folded into persist — no narrated findings to Neil
- [ ] Human-channel summary ≤200 words with no execution detail or file paths
- [ ] `ttal go <uuid>` ran cleanly in step 6

> **Reminder:** If persist-bound content leaked to the human channel, you burned Neil's context window.
