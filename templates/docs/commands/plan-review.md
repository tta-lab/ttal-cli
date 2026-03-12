---
name: plan-review
description: "Review a flicknote plan for issues before approving execution"
argument-hint: "<flicknote-id> [flicknote-id...]"
claude-code:
  allowed-tools:
    - Agent
opencode: {}
---

# Plan Review

Review flicknote plans before execution using the **plan-reviewer** subagent.

## Usage

```
/plan-review <flicknote-id>
/plan-review <id1> <id2> <id3>
```

## Workflow

### Single plan

Launch one **plan-reviewer** subagent:

```
Agent(subagent_type: "plan-reviewer", prompt: "Review the plan: flicknote get <id>")
```

### Multiple plans

Launch one **plan-reviewer** subagent per plan **in parallel**:

```
Agent(subagent_type: "plan-reviewer", prompt: "Review the plan: flicknote get <id1>")
Agent(subagent_type: "plan-reviewer", prompt: "Review the plan: flicknote get <id2>")
Agent(subagent_type: "plan-reviewer", prompt: "Review the plan: flicknote get <id3>")
```

When all complete, present a summary table:

```markdown
# Plan Review Summary

| Plan | Calibration | Verdict | Critical Issues |
|------|------------|---------|-----------------|
| <id> <title> | Just right | Ready | 0 |
| <id> <title> | Over-engineered | Needs revision | 2 |
```

Then show each plan's full review below the table.

## After Review

Present the verdict and let Neil or the routing agent decide:
- **Ready** — plan can proceed to `ttal task execute`
- **Needs revision** — use `/plan-triage` to categorize and fix issues, or route back to the design agent
- **Needs rethink** — plan has fundamental problems, needs brainstorming first

## Turns

Plans rarely pass on the first round. The expected cycle is:

```
plan-review → issues found → plan-triage or route to designer → revise → plan-review again
```

For subsequent rounds, **resume** the same subagent so it retains context from previous rounds:

```
Agent(resume: "<agentId from round 1>", prompt: "Plan has been revised. Re-review round 2: flicknote get <id>")
```

The resumed subagent remembers its previous findings and can track resolved/persisting/new issues automatically.

If the same critical issue persists for 2+ rounds, escalate to Neil rather than routing back to the designer again.
