---
name: sp-roleplay
description: Use when running long collaborative fiction or roleplay sessions (>5 turns) with another agent or human partner — when continuity, character voice, world-state, and narrative resolution must hold across many exchanges.
category: methodology
---

# Coherent Roleplay (Collaborative Fiction)

## Overview

Roleplay is the failure mode where forward-creative pull overwhelms backward-consistency check. The model writes a good next turn while losing track of facts established 3 turns ago. Without scaffolding, internal continuity errors compound non-linearly with turn count; with scaffolding, they go to near-zero.

The scaffolding is structured external state: a scratch file you maintain as you write, with sections for resolution, cast, per-character state, voice, terms, canon hedges, mechanism commits, and turn-by-turn fact logs. The model alone cannot hold this; the scratch + the discipline of reading it before each turn + writing to it after, together do.

This skill encodes that discipline. Five bug-types × five mitigations, applied in order.

**Announce at start:** "I am using sp-roleplay to maintain narrative coherence across this scene."

**First action:** Create the scratch file at your workspace (e.g. `templates/ttal/<agent>/roleplay-scratch.md`), copy the template from `scratch.template.md` in this skill directory, and fill in Phase 0 sections BEFORE writing the first in-scene turn.

## When to Use

Use this skill when:
- You're going to write more than ~5 turns of collaborative fiction with a partner
- The fiction has named characters, established objects, and a tracked world-state
- Continuity errors (object placement, character position, time of day, prior statements) would damage immersion
- Character voice integrity matters across the scene
- The fiction is set in a known canon (anime / film / novel) where canon-fidelity matters
- The fiction involves invented mechanisms (sci-fi physics, magic systems) that need internal consistency

Do NOT use this skill for:
- Single-turn jokes or replies in character
- Brief one-shot creative writing (<5 turns)
- Code or technical writing (use sp-write-test-plan, sp-research, etc.)

## Phase 0: Pre-write Resolution (BLOCKING)

**Hard rule: do not open the scene without filling these sections.** Skipping this phase produces 虎头蛇尾 (strong opening, weak ending) — the dominant failure mode of improvised long fiction.

Before turn 1, fill in the scratch:

1. **Resolution paragraph (REQUIRED)** — one paragraph describing:
   - What does this scene resolve? What's the dramatic question?
   - Where does each named character end up at scene close (state, decision, change)?
   - What artifact (object, statement, gesture) marks the resolution?
   - **If you can't write this paragraph, you don't have a story yet — STOP and design more before opening.**

2. **Setup** — date / time, location, weather, ambient details, canon timeline anchor.

3. **Cast register** — every character on stage, off stage, or named. For each:
   - Canonical name + aliases
   - Role (you play / partner plays / NPC) and stage status
   - Voice notes — register, vocabulary, address conventions, behavioral tells

4. **Per-character state board** (table) — for every named character:
   - `current_position` (geographical / spatial)
   - `current_posture` (sitting / standing / walking)
   - `last_action`
   - `holding` (objects in hand)
   - **Update this table after every turn.**

5. **Term glossary** — for non-native-language terms (Japanese for anime canon, technical vocab for sci-fi), pre-define each in scene-language. This prevents your partner from interrupting to query.

6. **Canon-uncertain hedges** — list every claim you're not 100% sure of. Mark each as:
   - `verified` — looked up, locked
   - `hedged` — will use vaguely in scene (e.g. "some years ago" instead of a specific year)
   - `guessed` — will use anyway, but verify if it becomes load-bearing

7. **Mechanism commits** — if the scene involves invented physics or mechanism (broadcast radius, magic system, time-travel rules), lock the parameters here BEFORE the mechanism appears in dialogue. Without this, you will conflate sub-mechanisms in later turns.

## Phase 1: Open the Scene

After Phase 0 is complete, write turn 1.

- Set physical scene with concrete sensory anchors. 3–5 specifics earn their space; more is over-writing.
- Establish at least one object per character that may persist across turns (callback potential).
- End with a clear hand-off — a question, gesture, or pause that lets your partner enter.

After writing turn 1, append a "Turn 1" entry to the turn-by-turn log in the scratch:
- Every fact committed (objects, positions, statements)
- Every dialogue line (exact wording)
- Update the per-character state board

## Phase 2: Per-Turn Discipline

For every turn (yours and your partner's):

### Before writing your turn
- Re-read the per-character state board
- Re-read the most recent 2–3 entries in the turn-by-turn log
- Re-read your Resolution paragraph — am I drifting from the destination?

### While writing your turn
- Cross-check each new claim against the canon-uncertain list. If a `guessed` claim is becoming load-bearing, stop and verify.
- For invented mechanisms, cross-check against Mechanism commits — don't add new constraints that contradict prior.

### Before sending NPC dialogue (BLOCKING)
- Scan the dialogue line by line.
- For each reference to player / partner state (position, posture, clothing, action), QUERY the state board.
- If NPC says "stop standing" but board says player is sitting — STOP and rewrite.
- This is the single highest-leverage check. Most position bugs come from NPC speech written without re-querying the player's actual state.

### After sending your turn
- Append turn entry to turn-by-turn log
- Update per-character state board
- If you discovered a contradiction mid-write, OUT-OF-FRAME correct (don't retcon)

## Phase 3: Mid-Scene Audit (every 5 turns)

Run a quick consistency scan:
- Re-read all turn entries
- Look for: object position drift, time inconsistency, voice drift, mechanism contradiction
- If found, OUT-OF-FRAME correct in the next turn — never silent retcon

## Phase 4: Scene Close

Before writing the final turn:

1. **Resolution check** — does the close land each character at the position your Resolution paragraph specified?
2. **Character integrity check** — is each character speaking in their established register? **Do not soften a hard character (a decisive military officer becoming sentimentally chatty, etc.) for emotional convenience.** Hard characters' restraint IS their tenderness; reverse it and you collapse the character.
3. **Plot stance commit** — if your scene has an open question (ritual vs hope, true vs false), commit to one. Both-sides-implied = wishful thinking; readers feel the lack of stance.
4. **Don't add new actors or mechanisms in the close** — close uses what's already on the table.

After closing, run the Closing Audit Checklist (see template).

## The 5 Bug Types × 5 Mitigations

| Bug type | Source | Mitigation |
|---|---|---|
| Internal continuity (object position, time-of-day) | No external state, model alone | Scratch turn-by-turn log + per-turn re-read |
| Mechanism contradiction (broadcast radius, magic rule) | Invented physics not pre-locked | Mechanism commits section, locked before scene |
| Player-state mismatch (NPC says "stop standing" while player sits) | NPC dialogue doesn't query player state | Per-character state board + NPC dialogue cross-check (Phase 2) |
| Character integrity drift (hard character softened for warmth) | Writer prioritizes emotional convenience over register | Voice notes locked in Phase 0 + Phase 4 character-integrity check |
| 虎头蛇尾 (strong open, weak end) | No resolution designed before opening | Resolution paragraph BLOCKING (Phase 0) |

The first three are mechanical (scratch + discipline catches them). The last two are craft (scratch records the constraint; the writer must respect it).

## Red Flags — STOP and Fix

- You're writing turn N+1 without having read the state board for turn N
- An NPC line describes the player doing X; your gut says "is X actually true right now?"; you're about to write it anyway
- You hit a contradiction mid-write and reach for in-scene retcon ("...wait, she misspoke")
- The scene is ~70% through and you don't know how it ends
- A character is about to do something that violates their voice notes; you're rationalizing
- You introduce a new mechanism in the close
- You're maintaining ambiguity on a plot question because both options feel emotionally appealing

In all cases: stop, OUT-OF-FRAME correct (or pause and design more), then continue.

## Honest Trade-offs

- Scratch overhead is ~10–15 sec per turn; scales linearly with turns. Worth it past ~5 turns; below that, overhead dominates.
- This skill catches **your** inconsistencies, not your partner's. Only what you commit is auditable.
- Sandbox-genre worlds (Ghibli, Mushishi, Aria) have lower canon-fidelity load; tight-canon worlds (Steins;Gate, EVA) have higher. Adjust Phase 0 canon-uncertain effort accordingly.
- Some craft-level failures (character integrity, plot stance) require writing skill the scratch can't substitute for. The scratch makes the constraint explicit; you still have to honor it.

## World-Framework × Story-Structure Axis

For choosing roleplay material:

| | Sandbox structure | Plot structure |
|---|---|---|
| **Tight world** | GitS SAC, Cowboy Bebop, Detective Conan, Frieren post-canon — **sweet spot**: detail density + invention freedom | Steins;Gate, EVA, FMA — high-fidelity emulation only, low invention |
| **Loose world (slice-of-life)** | Aria, Mushishi, Yotsuba, generic AU — pure relaxation, low canon load | (rare) |

For both freedom AND coherence: pick a tight-world + sandbox-structure setting.
For deep emotional landing: pick tight-world + plot-canon, accept invention constraint.
For pure relaxation: pick loose-world + sandbox, lower the canon-fidelity bar.

## Reference: Scratch Template

See `scratch.template.md` in this skill directory. Copy at scene start, fill Phase 0 sections, append turn entries as you go.
