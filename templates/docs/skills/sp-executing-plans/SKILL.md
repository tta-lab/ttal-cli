---
name: sp-executing-plans
description: Use when you have a written implementation plan to execute in a separate session with review checkpoints
---

# Executing Plans

## Overview

Load plan, review critically, execute all tasks continuously, create PR when done.

**Core principle:** Plans are pre-approved by the architect. Execute them fully without pausing for feedback. Stop only when blocked.

**Announce at start:** "I'm using the executing-plans skill to implement this plan."

## The Process

### Step 1: Load and Review Plan

1. Read the plan file (path is in the task annotation or task description)
2. Review critically — identify any questions or concerns
3. If concerns: Raise them before starting
4. If no concerns: Create TodoWrite and proceed

### Step 2: Execute All Tasks

Execute every task in the plan **continuously** — do not pause between tasks or batches.

For each task:
1. Mark as in_progress
2. Follow each step exactly (plan has bite-sized steps)
3. Run verifications as specified (build, test)
4. Commit as specified in the plan
5. Mark as completed
6. Move to the next task immediately

### Step 3: Complete Development

After all tasks complete and verified:

1. **Verify all tests pass**

2. **Push:**
   ```bash
   git push
   ```

3. **Create PR:**
   ```bash
   ttal pr create "PR title" --body "description of changes"
   ```

## When to Stop and Ask for Help

**STOP executing immediately when:**
- Hit a blocker mid-batch (missing dependency, test fails, instruction unclear)
- Plan has critical gaps preventing starting
- You don't understand an instruction
- Verification fails repeatedly

**Ask for clarification rather than guessing.**

## When to Revisit Earlier Steps

**Return to Review (Step 1) when:**
- Partner updates the plan based on your feedback
- Fundamental approach needs rethinking

**Don't force through blockers** - stop and ask.

## Remember
- Review plan critically first
- Follow plan steps exactly
- Don't skip verifications
- Reference skills when plan says to
- Execute continuously — do NOT pause between tasks for feedback
- Stop only when blocked (missing dependency, test fails, unclear instruction)
- Never start implementation on main/master branch without explicit user consent

## Integration

**Required workflow skills:**
- **sp-writing-plans** - Creates the plan this skill executes
- **knowledge** - Vault conventions (frontmatter, folder structure)
