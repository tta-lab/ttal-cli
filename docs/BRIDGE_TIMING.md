# Bridge Timing: Stop Hook vs JSONL Flush

## The Problem

The `ttal bridge` Stop hook needs to read the current turn's assistant text from
the JSONL transcript. However, the Stop hook fires **before** CC flushes the
current turn's text to disk. On first read, the JSONL only contains previous
turns.

## How We Detect Staleness

CC writes a `stop_hook_summary` entry to the JSONL **after** all Stop hooks
complete. This gives us a staleness signal:

- **Previous turn's text**: followed by a `stop_hook_summary` entry
- **Current turn's text**: NOT followed by a `stop_hook_summary` (because our
  hook is still running, so the summary hasn't been written yet)

The bridge scans backwards through the last 50 JSONL lines. When it finds an
assistant text block, it checks if any `stop_hook_summary` appears after it. If
so, the text is stale (from a previous turn) and we return empty.

## The Retry Loop

Since the current turn may not be flushed yet, we retry up to 5 times with
200ms delays:

1. Read JSONL tail, look for fresh (non-stale) assistant text
2. If none found, wait 200ms and try again
3. After 5 attempts, give up silently

In practice, **one retry is usually enough** — CC flushes within ~200ms of the
hook firing.

## What We Don't Know

We don't know for sure if CC waits for all hooks to exit before writing
`stop_hook_summary`. The docs don't specify this ordering. Our approach assumes:

> CC will eventually flush the current turn's assistant text, and we can
> distinguish it from previous turns because `stop_hook_summary` hasn't been
> written yet (our hook is still running).

This assumption holds empirically — the bridge reliably sends fresh text with
different character counts each turn, confirmed via `~/.ttal/bridge.log`.

## If This Breaks

If CC changes the ordering (e.g., writes `stop_hook_summary` before hooks
finish, or stops writing it entirely):

- **Symptom**: bridge sends stale/repeated text, or sends nothing
- **Check**: `~/.ttal/bridge.log` for retry patterns
- **Fallback**: Replace staleness detection with a simple sleep (500ms worked
  empirically in early testing)
- **Ideal fix**: CC provides `last_assistant_message` in the Stop hook stdin
  payload, eliminating the need to parse JSONL entirely

## Related Files

- `internal/bridge/bridge.go` — `extractCurrentTurnText()` and retry loop
- `internal/bridge/jsonl.go` — JSONL entry types including `Subtype` field
- `docs/plans/2026-02-18-bridge-design.md` — Full bridge design
