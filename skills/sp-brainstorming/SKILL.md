---
name: sp-brainstorming
description: Use when a request needs collaborative exploration of intent, constraints, alternatives, or design before the direction is settled.
---

# Brainstorming

Turn an unclear idea into an agreed design through conversation. This skill ends when the design is recorded; it never starts planning or implementation.

## Process

1. Inspect the current project, prior FlickNote notes, and relevant code.
2. Ask one focused question at a time. Clarify purpose, constraints, success criteria, and anti-goals.
3. Offer two or three viable approaches with trade-offs and a recommendation.
4. Present the design in small sections and confirm each section before continuing.
5. Cover architecture, data flow, failure handling, testing, and rollout when relevant.

## Persist the Design

After the user approves the direction, save the agreed design to the `orientation` project:

flicknote add --project orientation

The note should contain:

- What and why
- Chosen approach and rejected alternatives
- Constraints and anti-goals
- Success criteria
- Open risks

Return the FlickNote ID and a short summary. Stop there. Do not invoke or recommend another skill automatically.

For a small or mechanical change where the conversation itself is enough, skip the note and state the agreed direction.

## Rules

- One question per turn
- Search before asking for discoverable facts
- Prefer the smallest design that meets the goal
- Do not implement while brainstorming
- Do not assume the user wants a plan or execution next
