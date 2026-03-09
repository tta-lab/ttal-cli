---
name: sp-research
description: Use when conducting structured research on a task — multi-source investigation with structured output
---

# Research

## Overview

Conduct structured, multi-source research that produces actionable findings. Research is not aggregation — it's synthesis. Every finding should connect to a "so what?" that helps the team make decisions.

**Announce at start:** "I'm using the research skill to investigate this."

## Research Quality Standards

- **Multi-source:** Combine web search, web fetch, Context7 docs, and local source code
- **Synthesis:** Analyze and provide insights, not just collect links
- **Actionable:** Include recommendations and concrete next steps
- **Sourced:** Always cite sources with links
- **Honest:** If research hits a dead end, document why — don't force conclusions

## Research Process

1. **Understand the question** — read the task description and annotations. What decision does this research inform?
2. **Survey** — web search for overview, identify key sources
3. **Deep dive** — web fetch important docs, read source code if needed, query Context7 for library docs
4. **Synthesize** — connect findings, identify trade-offs, form recommendations
5. **Write findings** — structured document with clear sections
6. **Save and annotate** — save findings using the storage method configured for your team, annotate the task

## Findings Document Structure

Every research doc should have:

```markdown
# Research: [Topic]

## Question
[What decision does this inform? What are we trying to learn?]

## Context
[Why this matters, what prompted the investigation]

## Findings
[Multi-source synthesis — not a link dump]

## Trade-offs
[If comparing approaches: pros/cons of each]

## Recommendation
[Clear recommendation with reasoning. "We should X because Y."]

## Open Questions
[What we still don't know, if anything]

## Sources
- [Source name](url) — brief note on what it contributed
```

Not every section is required — skip what doesn't apply. But **Question**, **Findings**, and **Recommendation** are always needed.

## After Research Is Complete

1. **Save findings** — use the storage method configured for your team
2. **Annotate the task** — reference the saved findings so others can find them
3. **Hand off to design:** `task <uuid> modify -research +design` — moves task to the design phase
4. **Never mark done** — research tasks stay open and flow through the pipeline

If research is **partial** (ran out of time/tokens), annotate what you have and keep the task as `+research` pending.

If research **failed** (dead end, question unanswerable), annotate why and keep the task as `+research` pending for review.

## Source Priority

1. **Official docs** — always preferred over blog posts
2. **Context7** — up-to-date library documentation with code examples
3. **Source code** — read the actual implementation when docs are unclear
4. **Web search** — for overview and discovering sources
5. **Blog posts** — only when official sources are insufficient

## Remember

- Research informs decisions — always end with a recommendation
- One task per session — go deep, not wide
- Cite everything — unsourced claims are useless
- If tools fail, stop and report — don't work around silently
- Partial findings are better than no findings — save what you have
