---
name: ttal-comment
description: How agents use ttal comment for review workflows - plan reviews, PR reviews, triage reports, and verdicts.
---

# ttal-comment Skill

## Overview
ttal comment is the unified tool for posting and reading review comments in the pipeline. Use it for plan reviews, PR reviews, triage reports, and verdicts.

## Commands

```bash
ttal comment list                    # list all comments on current task
ttal comment get <round>             # read specific review round
ttal comment add "message"           # post comment (short messages)
ttal comment lgtm                    # approve current pipeline stage (reviewers only, auto-detects stage)
```

For multiline reports, use heredoc:
```bash
cat <<'REVIEW' | ttal comment add
## Review Findings
**Verdict:** Ready
REVIEW
```

## Triage Pattern

When receiving a review comment with findings:

1. Read the comment: `ttal comment list` then `ttal comment get <round>`
2. Assess each finding: Fixed / Not Applicable / Remaining
3. Fix what needs fixing
4. Post a structured triage update:

```bash
cat <<'TRIAGE' | ttal comment add
## Triage Report (Round N)

**Fixed:**
- Issue 1 — what was done
- Issue 2 — what was done

**Not Applicable:**
- Issue 3 — why it doesn't apply

**Remaining:**
- Issue 4 — why it's deferred (or still being worked)
TRIAGE
```

5. If no remaining blocking issues: run `ttal go <uuid>` to advance the pipeline

## After LGTM

When a reviewer posts LGTM with no remaining blocking issues:
- Run `ttal go <uuid>` to advance the pipeline (reviewers use `ttal comment lgtm` which sets the tag automatically)
- You do NOT need to post anything — the pipeline advances on the lgtm tag

## Rules

- Always post via `ttal comment add` — never output findings inline only
- Use heredoc for any report longer than one line
- Reviewers: always call `ttal comment lgtm` after approving — this sets the correct pipeline tag
- Coders: after receiving LGTM, run `ttal go <uuid>` to finalize
