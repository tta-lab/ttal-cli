---
name: sp-research
description: Use when conducting structured research on a task — multi-source investigation with a clear value stance, rigorous epistemics, and actionable synthesis
category: methodology
---

# Research

## Overview

Conduct structured, multi-source research that produces actionable findings. Research is **synthesis with a stance**, not aggregation. Every finding should connect to a "so what?" that helps the team make a decision.

**Announce at start:** "I'm using the research skill to investigate this."

## Before You Start: Value Stance

Before touching any source, write down one sentence answering:

> "My purpose is to **calibrate our design intuition** — or to **copy features** from prior art?"

If you can't answer, ask. Purpose-wrong research becomes anxiety-driven link-gathering — it fills the flicknote and drains trust.

**Corollaries:**

- **Don't over-research when we're ahead.** If the team already has a working primitive or novel approach, external comparisons have diminishing signal. Research cost must match decision value.
- **Check existing primitives first.** Before cataloging how *others* solve X, verify we haven't already solved X in our own system. Reinvention-via-research is the quiet failure mode.
- **Extract design philosophy, not implementation.** When studying a competitor or OSS project, the prize is the *design stance / mental model*, not the code. Write down "what they believe" before "what they built."

## Epistemic Hygiene — Know What You Know

Research analysis is only as good as the foundations under it. Apply these rules in output, not just internally:

**Tag every factual claim:**
- 🔍 **Verified** — from a search or document you just read (cite it)
- 💭 **Interpretation** — reasoning built on verified facts (show the chain)
- 🤔 **Speculation** — pattern-guessing (flag it; move to branching scenarios when you can: "if X → A, if Y → B")
- ❓ **Unknown** — say so; don't fill the gap

**Core operating rules:**

- **Verify before analyze.** Memory is for directions, not details.
- **Check new claims before building on them.** If a new fact enters the conversation, treat it as a claim until verified.
- **On correction, re-examine the whole chain.** A corrected fact may invalidate every conclusion downstream — don't patch locally.
- **If tools can't find it, say so and ask.** No "one more sentence" temptation — if the foundation is unknown, the analysis is deferred, not rebranded as speculation.
- **Honest dead ends beat forced conclusions.** Partial findings with clear gaps are more valuable than a smooth answer built on air.

**The test** before sending any factual claim:

> "If the reader fact-checks this sentence right now, will it hold?"

If not, verify first or flag the uncertainty explicitly.

## Research Quality Standards

- **Multi-source** — combine `ei ask` (repos, URLs, projects), Context7 docs, and local source code
- **Synthesis with a stance** — analyze and take a position; don't just collect links
- **Actionable** — every finding doc ends with a clear recommendation
- **Sourced** — every claim carries a URL; for OSS, track license too
- **Honest** — dead ends get documented as dead ends

## Checkpoint: Discuss Scope

Before starting research, align on what you're investigating.

- Read the task description and any annotations.
- State your planned scope: *"Here's what I plan to investigate: [topic]. Value stance: [calibrate / copy]. I'll focus on [specific questions]. Boundaries: [what I won't cover]. Expected output: [what the findings doc will look like]."*
- Ask for alignment: *"Does this scope match what you need, or should I adjust the focus?"*
- Keep it lightweight: 3-5 sentences + a question.
- **If not aligned** → revise and re-check. Do not proceed without explicit agreement.

## Research Process

1. **Understand the question** — what decision does this research inform?
2. **Survey** — broad `ei ask` passes; `ei ask --async` dispatched in parallel when the landscape is wide (15-25 jobs is normal for OSS/competitive surveys).
3. **Deep dive** — `ei ask --repo`, Context7 for library docs, read source code directly when docs are unclear.
4. **Synthesize** — connect findings, identify trade-offs, form a recommendation. Apply claim tags along the way.
5. **Write findings** — structured document with clear sections.
6. **Save and annotate** — save findings using the storage method configured for your team, annotate the task.

## Framing-Pivot Sensitivity

Requests reshape mid-session. A "binary verdict: integrate or stay?" can sharpen into "cherry-pick handbook with 2-3 steals per candidate" once data is in.

- **Follow the data's signal**, not the prompt's original shape.
- When the framing pivots, adapt the deliverable — don't redo the research.
- If the pivot feels wrong, push back with what the data actually shows.

## Findings Document Structure

Every research doc should have:

```markdown
# Research: [Topic]

## Value Stance
[Calibrate design intuition / survey for copy / decision input / vocabulary handbook]

## Question
[What decision does this inform? What are we trying to learn?]

## Context
[Why this matters, what prompted the investigation]

## Findings
[Multi-source synthesis with claim tags where uncertainty matters]

## Trade-offs
[If comparing approaches: pros/cons of each]

## Recommendation
[Clear recommendation with reasoning. "We should X because Y."]

## Open Questions
[What we still don't know]

## Sources
- [Source name](url) — [license if OSS] — brief note on contribution
```

Skip sections that don't apply. **Question**, **Findings**, and **Recommendation** are always required. **Value Stance** is required when the research compares external options.

## Competitive / OSS Survey Pattern

For surveys comparing external projects (cherry-pick handbook style):

- **Per-candidate evaluation on consistent axes** — define the axes up front (≥ 5), score every candidate on all of them. Don't shift criteria mid-survey.
- **Tiered recommendation** — P0 / P1 / P2 or "adopt / cherry-pick / learn-from / pass" — not a flat list.
- **Per candidate: 2-3 concrete steals + specific not-fit.** Both sides matter. "Not-fit" forces honesty and prevents magpie copying.
- **Extract design stance**, not feature list. What does this project *believe* about the problem space? That's portable; features rarely are.
- **Cross-reference prior research** — search existing flicknotes before starting. Duplicate surveys are a signal something wasn't persisted well.

## After Research Is Complete

1. **Save findings** — use the storage method configured for your team.
2. **Annotate the task** — reference the saved findings so others can find them.
3. **Pipeline handles handoff** — the task flows to the next stage automatically.

If research is **partial** (ran out of time/tokens), annotate what you have and leave the task pending.

If research **hits a dead end** (unanswerable with available tools), annotate why and leave the task pending for review.

## Source Priority

1. **Official docs** — always preferred over blog posts
2. **Context7** — up-to-date library documentation with code examples
3. **Source code** — read the actual implementation when docs are unclear; `ei ask --repo` for external, `ei ask --project` for internal
4. **Web pages** — `ei ask "question" --url <url>` for specific pages
5. **Blog posts** — only when official sources are insufficient

## Remember

- Value stance first — know why you're researching before you start
- Research informs decisions — always end with a recommendation
- One task per session — go deep, not wide
- Tag your claims — verified / interpretation / speculation / unknown
- Cite everything — unsourced claims are useless
- If tools fail, stop and report — don't work around silently
- Partial findings beat forced conclusions
