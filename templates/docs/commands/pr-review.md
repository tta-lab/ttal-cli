---
name: pr-review
description: "Comprehensive PR review using 7 specialized agents"
argument-hint: "[aspects: code|errors|tests|comments|types|principles|simplify|all]"
claude-code:
  allowed-tools:
    - Bash
    - Glob
    - Grep
    - Read
    - Task
---

# Comprehensive PR Review

Run a comprehensive pull request review using multiple specialized agents, each focusing on a different aspect of code quality.

**Review Aspects (optional):** "$ARGUMENTS"

## Environment

This command always runs in a **git worktree** with the branch already checked out and all code local. Do not run `git pull`, `git fetch`, `git checkout`, or any network git operations. Just use `git diff` and `git log` against what's already here.

## Review Workflow

1. **Determine Review Scope**
   - Check git status to identify changed files
   - Parse arguments to see if user requested specific review aspects
   - Default: Run all applicable reviews

2. **Available Review Aspects:**

   - **code** - General code review for project guidelines
   - **errors** - Check error handling for silent failures
   - **tests** - Review test coverage quality and completeness
   - **comments** - Analyze code comment accuracy and maintainability
   - **types** - Analyze type design and invariants (if new types added)
   - **principles** - Check for DRY, SOLID, KISS, YAGNI, guard clauses violations
   - **simplify** - Simplify code for clarity and maintainability
   - **all** - Run all applicable reviews (default)

3. **Identify Changed Files**
   - Run `git diff --name-only` to see modified files
   - Identify file types and what reviews apply

4. **Determine Applicable Reviews**

   Based on changes:
   - **Always applicable**: pr-code-reviewer (general quality)
   - **If error handling changed**: pr-silent-failure-hunter
   - **If test files changed**: pr-test-analyzer
   - **If comments/docs added**: pr-comment-analyzer
   - **If types added/modified**: pr-type-design-analyzer
   - **Always applicable**: pr-principles-reviewer (DRY, SOLID, KISS, YAGNI)
   - **After passing review**: pr-code-simplifier (polish and refine)

5. **Launch Review Agents**

   Launch each applicable specialist using the Task tool with `subagent_type`:
   - `subagent_type: "pr-code-reviewer"`
   - `subagent_type: "pr-silent-failure-hunter"`
   - `subagent_type: "pr-code-simplifier"`
   - `subagent_type: "pr-comment-analyzer"`
   - `subagent_type: "pr-test-analyzer"`
   - `subagent_type: "pr-type-design-analyzer"`
   - `subagent_type: "pr-principles-reviewer"`

   Launch independent specialists in parallel when possible.

6. **Aggregate Results**

   After agents complete, summarize:
   - **Critical Issues** (must fix before merge)
   - **Important Issues** (should fix)
   - **Suggestions** (nice to have)
   - **Positive Observations** (what's good)

7. **Provide Action Plan**

   Organize findings:
   ```markdown
   # PR Review Summary

   ## Critical Issues (X found)
   - [agent-name]: Issue description [file:line]

   ## Important Issues (X found)
   - [agent-name]: Issue description [file:line]

   ## Suggestions (X found)
   - [agent-name]: Suggestion [file:line]

   ## Strengths
   - What's well-done in this PR

   ## Recommended Action
   1. Fix critical issues first
   2. Address important issues
   3. Consider suggestions
   4. Re-run review after fixes
   ```

## Usage Examples

**Full review (default):**
```
/pr-review
```

**Specific aspects:**
```
/pr-review tests errors
# Reviews only test coverage and error handling

/pr-review comments
# Reviews only code comments

/pr-review simplify
# Simplifies code after passing review
```

## Agent Descriptions

**pr-code-reviewer**:
- Checks CLAUDE.md compliance
- Detects bugs and issues
- Reviews general code quality
- Confidence-gated (>= 80/100)

**pr-silent-failure-hunter**:
- Finds silent failures
- Reviews catch blocks
- Checks error logging

**pr-test-analyzer**:
- Reviews behavioral test coverage
- Identifies critical gaps
- Evaluates test quality

**pr-comment-analyzer**:
- Verifies comment accuracy vs code
- Identifies comment rot
- Checks documentation completeness

**pr-type-design-analyzer**:
- Analyzes type encapsulation
- Reviews invariant expression
- Rates type design quality

**pr-principles-reviewer**:
- Checks DRY, SOLID, KISS, YAGNI
- Flags guard clause opportunities
- Reviews cyclomatic complexity
- Confidence-gated (>= 80/100)

**pr-code-simplifier**:
- Simplifies complex code
- Improves clarity and readability
- Applies project standards
- Preserves functionality

## Tips

- **Run early**: Before creating PR, not after
- **Focus on changes**: Agents analyze git diff by default
- **Address critical first**: Fix high-priority issues before lower priority
- **Re-run after fixes**: Verify issues are resolved
- **Use specific reviews**: Target specific aspects when you know the concern
