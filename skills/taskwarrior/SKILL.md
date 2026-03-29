---
name: taskwarrior
description: Task management using taskwarrior with smart notifications and markdown context integration.
---

# Taskwarrior Skill

**Purpose:** Task management using taskwarrior with smart notifications and markdown context integration.

**Philosophy:** Minimal wrapper around taskwarrior CLI. Let taskwarrior do what it does best, add Python helpers only for custom workflows.

## Quick Start

### Creating Tasks

```bash
# Simple task
task add "Fix bug in worker status" project:clawd +bugfix priority:H due:tomorrow

# Task with inline notes
task add "Research taskwarrior hooks" project:clawd +research priority:M
task 1 annotate "Check: on-modify, on-exit hooks
Look at examples in taskwarrior docs
Test with simple echo script first"

# Task with external documentation
task add "Design OAuth refresh flow" project:clawd +design priority:H due:3days
task 1 annotate "Design: ~/clawd/docs/plans/2026-01-31-oauth-refresh-design.md"
```

### Managing Tasks

```bash
# Mark as active
task 1 start

# Add notes
task 1 annotate "Additional context here"

# Complete task
task 1 done

# Delete task (no longer relevant)
task 1 delete

# Defer task for a week
task 1 modify wait:1week
```

### Viewing Tasks

```bash
# List all pending tasks
task

# List by project
task project:clawd list

# List by tag
task +bugfix list

# View specific task
task 1

# View task with markdown context
~/clawd/skills/taskwarrior/scripts/task-open.py 1
```

## Task Structure

### Core Attributes

- **Description** (required): Short, clear task title
- **Project**: Hierarchical grouping (e.g., `clawd`, `clawd.skills`, `guion.flicknote`)
- **Tags**: Multiple single-word tags (e.g., `+bugfix`, `+research`, `+implementation`)
- **Priority**: H/M/L for importance
- **Due date**: When task should be completed
- **Annotations**: Multi-line notes or file references

### Three Task Patterns

**Pattern 1: Simple tasks** - No extra context needed
```bash
task add "Update README" project:clawd priority:L
```

**Pattern 2: Tasks with inline notes** - Context fits in annotations
```bash
task add "Investigate performance issue" project:clawd +bugfix priority:H
task 1 annotate "User reports slow response on large datasets
Check database query optimization
Profile API endpoint response times"
```

**Pattern 3: Tasks with external docs** - Complex planning needs separate file
```bash
task add "Design new feature architecture" project:clawd +design priority:H
task 1 annotate "Design: ~/clawd/docs/plans/2026-01-31-feature-design.md"
# View with: ~/clawd/skills/taskwarrior/scripts/task-open.py 1
```

### File Reference Formats

The `task-open.py` script recognizes these annotation patterns:

```
Design: ~/clawd/docs/plans/2026-01-31-design.md
Doc: ~/clawd/docs/architecture.md
Reference: ~/clawd/docs/api-spec.md
File: ~/path/to/any-doc.md
```

All paths are expanded automatically (`~` → home directory).

## Status Management

- **Pending** → Default state for new tasks
- **Active** → `task N start` when currently working on it
- **Completed** → `task N done` when finished
- **Deleted** → `task N delete` if no longer relevant
- **Waiting** → `task N modify wait:1week` to defer

## Task Dependencies

Taskwarrior has built-in dependency tracking. **Never duplicate dependency information in descriptions or annotations.**

### ✅ Correct Way

```bash
# Create tasks
task add "Fix authentication bug" project:clawd +bugfix priority:H
task add "Deploy authentication fix" project:clawd +deployment priority:H

# Set dependency (task 2 depends on task 1)
task 2 modify depends:1

# Annotations should have context, not dependency info
task 2 annotate "Need staging environment approval before production deploy"
```

**Result:**
```bash
task list
# Shows:
# 1 [H] Fix authentication bug
# 2 [H] Deploy authentication fix (BLOCKED)
```

Task 2 automatically shows as BLOCKED until task 1 is completed.

### ❌ Wrong Way - Don't Do This!

```bash
# BAD: Putting dependency info in annotations
task 2 annotate "Blocked by: Task #1 (authentication bug fix)"
task 2 modify depends:1  # Already tracked by taskwarrior!
```

**Why this is bad:**
- Duplicates information taskwarrior already tracks
- Annotations become stale when dependencies change
- Harder to query/filter by actual dependency status
- Violates DRY (Don't Repeat Yourself)

### Managing Dependencies

```bash
# View dependencies
task 2 info  # Shows "Depends on: 1"

# Remove dependency
task 2 modify depends:

# Multiple dependencies
task 3 modify depends:1,2  # Task 3 depends on both 1 and 2

# View all blocked tasks
task +BLOCKED list
```

### Helper Script for Workers

When working on a task, you can easily create a follow-up task that depends on the current one:

```bash
# From within a worker session:
create_dependent_task.py "Add timeouts to subprocess calls" --priority H
```

**What it does:**
- Automatically finds current task from `$ZELLIJ_SESSION_NAME`
- Creates new task with dependency on current task
- Inherits project from current task
- New task is BLOCKED until current task completes

**Example workflow:**
```bash
# Current task: "Migrate to taskwarrior UDAs"
create_dependent_task.py "Add error handling improvements" --priority M

# Output:
# ✓ Created dependent task: 040796ee...
#   Description: Add error handling improvements
#   Priority: M
#   Depends on: e65b0ee5...
#
# The new task will be blocked until the current task completes.
```

This is much easier than manually querying the current task UUID and setting dependencies!

### When Task Completes

```bash
task 1 done
# Task 2 automatically becomes unblocked!
```

**Rule:** Use `depends:` for dependencies, use annotations for context that taskwarrior doesn't track.

### ⚠️ Important: Task IDs vs UUIDs

**Task IDs change, UUIDs are permanent.**

- Task IDs (1, 2, 3...) are NOT stable - they change when tasks complete
- UUIDs are permanent identifiers that never change
- **Never reference task IDs in annotations** - they become invalid when IDs shift
- Dependencies use UUIDs internally (safe to use `depends:1`)

**❌ Bad - Don't do this:**
```bash
task add "Implement feature" project:clawd
task 5 annotate "Depends on task #3 to be completed first"
# ↑ If task 3 completes, it might become task 2, reference breaks!
```

**✅ Good - Use depends: field:**
```bash
task add "Implement feature" project:clawd
task 5 modify depends:3  # Uses UUID internally, safe!
task 5 annotate "Requires authentication system to be in place"
# ↑ Describes WHY, not WHAT task number
```

**When referencing other tasks:**
- In `depends:` field → Use task ID (converts to UUID)
- In annotations → Describe the dependency, don't use task numbers
- For permanent references → Use UUID directly if needed

## Smart Notifications

Two daily checks (8AM/2PM Asia/Taipei) via `check-urgent-tasks.py`:

### Tier 1: Urgent Tasks

Always shown if they exist:
- **Overdue**: Past their due date
- **Due soon**: Due in next 3 days
- **High-priority stale**: High priority, created 3+ days ago
- **Blocked ready**: Dependencies resolved (future)

### Tier 2: Suggestions

Shown if nothing urgent:
- **High-value**: High priority, recently created
- **Quick wins**: Unblocked, ready to start
- **Fresh ideas**: Added in last 48 hours
- **Random element**: Occasionally surface forgotten tasks

### Example Output

**Nothing urgent:**
```
✅ No urgent tasks!

💡 Suggested tasks to consider:

HIGH VALUE:
  #12 [H] Design caching layer (added yesterday)

QUICK WINS:
  #23 [L] Update README with new examples (no dependencies)

FRESH IDEAS:
  #25 [M] Explore Effect.ts error handling (added 1 day ago)
```

**Has urgent:**
```
🔴 Urgent Tasks Need Attention:

OVERDUE (1):
  #5 [H] Fix authentication bug (due 2 days ago)

DUE TODAY:
  #8 [M] Review PR #234

💡 Also consider:
  #12 [H] Design caching layer (when you have time)
```

## Built-in Reports

Taskwarrior has excellent built-in reporting:

```bash
# Task status breakdown by project
task summary

# Database statistics
task stats

# Visual burndown charts
task burndown.weekly
task burndown.monthly

# Task history over time
task history
```

## Common Workflows

### Daily Workflow

1. Check notifications (automatic at 8AM/2PM)
2. Review pending tasks: `task`
3. Start working on task: `task N start`
4. Complete task: `task N done`

### Planning Workflow

1. Create task with external doc reference:
   ```bash
   task add "Implement new feature" project:clawd +implementation priority:H
   task annotate "Design: ~/clawd/docs/plans/2026-01-31-feature-design.md"
   ```

2. Write detailed design in markdown file

3. View complete context:
   ```bash
   ~/clawd/skills/taskwarrior/scripts/task-open.py 1
   ```

4. Start implementation:
   ```bash
   task 1 start
   ```

### Weekly Review

```bash
# See what was completed this week
task end.after:today-7days completed

# Check summary by project
task summary

# Review burndown
task burndown.weekly
```

## Helper Scripts

### check-urgent-tasks.py

**Purpose:** Smart notification checker for cron

**Usage:**
```bash
~/clawd/skills/taskwarrior/scripts/check-urgent-tasks.py
```

**Exit codes:**
- 0: Clean or suggestions only
- 1: Urgent tasks exist

**Run manually:**
```bash
# Test notifications
~/clawd/skills/taskwarrior/scripts/check-urgent-tasks.py
echo $?  # Check exit code
```

### task-open.py

**Purpose:** Display task with inline markdown documentation

**Usage:**
```bash
~/clawd/skills/taskwarrior/scripts/task-open.py <task_id>
```

**Example:**
```bash
# View task #5 with all referenced documentation
~/clawd/skills/taskwarrior/scripts/task-open.py 5
```

## Cron Integration

Notifications run automatically twice daily:

- **Morning check**: 8:00 AM Asia/Taipei
- **Afternoon check**: 2:00 PM Asia/Taipei

Setup commands (already configured):
```bash
openclaw cron add "Morning task check" \
  --cron "0 8 * * *" \
  --session main \
  --system-event "Run ~/clawd/skills/taskwarrior/scripts/check-urgent-tasks.py" \
  --tz "Asia/Taipei"

openclaw cron add "Afternoon task check" \
  --cron "0 14 * * *" \
  --session main \
  --system-event "Run ~/clawd/skills/taskwarrior/scripts/check-urgent-tasks.py" \
  --tz "Asia/Taipei"
```

## Data Location

- **Data directory:** `~/.task/`
- **Database:** `~/.task/taskchampion.sqlite3`
- **Config:** `~/.taskrc` (uses defaults)

Taskwarrior works out of the box, no custom configuration needed.

## Tips and Best Practices

### Task Descriptions

- Keep descriptions short and action-oriented
- Start with verb: "Fix", "Implement", "Research", "Design"
- Be specific but concise

**Good:**
- "Fix authentication timeout bug"
- "Implement user profile caching"
- "Research OAuth 2.1 refresh flow"

**Bad:**
- "Bug" (too vague)
- "Need to look into the authentication system because users are reporting timeouts" (too long, use annotations)

### Projects and Tags

**Projects** are hierarchical:
```bash
task add "..." project:clawd
task add "..." project:clawd.skills
task add "..." project:guion.flicknote
```

**Tags** are flat and combinable:
```bash
task add "..." +bugfix +urgent
task add "..." +research +documentation
```

### Priorities

- **H (High)**: Must be done soon, important
- **M (Medium)**: Should be done, normal priority
- **L (Low)**: Nice to have, can wait
- **None**: No priority set

### Due Dates

Be realistic with due dates:
```bash
due:today
due:tomorrow
due:3days
due:1week
due:2026-02-15
```

Overuse of due dates leads to "due date fatigue". Only set when genuinely time-sensitive.

### Annotations

Multiline annotations work:
```bash
task 1 annotate "First line
Second line
Third line"
```

Link to files for complex context:
```bash
task 1 annotate "Design: ~/clawd/docs/plans/2026-01-31-design.md"
```

## Worker Lifecycle Hooks

Automated worker management using taskwarrior hooks that integrate with OpenClaw agents.

### Overview

When you start a task (`task N start`), a worker is automatically spawned. When you complete a task (`task N done`), the worker is automatically cleaned up based on PR status.

### How It Works

**Task Start → Worker Spawn:**
1. Hook detects task start
2. Sends task context to `worker-lifecycle` agent
3. Agent analyzes task and derives worker name/project
4. Agent calls `ttal worker spawn`
5. Task gets annotated with `Worker: <session-name>`
6. Telegram notification sent

**Task Complete → Auto Cleanup:**
1. Hook detects task completion
2. Checks for `Worker:` annotation
3. If PR merged: auto-cleanup (session + worktree + branch deleted)
4. If PR not merged: agent decides (keep or force cleanup)
5. If no worker: silent log
6. Telegram notification sent

### Installation

The hooks are managed by **chezmoi** in `~/clawd/dotfiles/dot_task/hooks/`:

```bash
# Hooks are automatically applied by chezmoi
chezmoi apply ~/.task/hooks

# Verify hooks are installed
task diagnostics | grep hooks
# Should show: on-modify-worker-lifecycle

# To edit hooks:
# 1. Edit in dotfiles: ~/clawd/dotfiles/dot_task/hooks/
# 2. Apply changes: chezmoi apply ~/.task/hooks
# 3. Commit to dotfiles repo
```

**Note:** Hooks are no longer symlinked. They are real files managed by chezmoi, which allows for better version control and multi-machine sync.

### Configuration

The `worker-lifecycle` OpenClaw agent handles spawn decisions and cleanup.

```bash
# Check agent exists
openclaw agents list
# Should show: worker-lifecycle

# View agent workspace
ls ~/.openclaw/agents/worker-lifecycle/
```

Agent configuration in `~/.openclaw/agents/worker-lifecycle/SOUL.md` defines:
- Worker naming conventions (kebab-case, max 30 chars)
- Project selection logic
- Cleanup decision criteria

### Monitoring

```bash
# View hook logs
tail -f ~/.task/hooks.log

# Check worker tracking
cat ~/.clawd-zellij/worker-tracking.json
```

### Example Workflow

```bash
# Start a task
task add "Implement user profile" project:clawd +feature priority:H
task 1 start

# Hook automatically:
# - Calls worker-lifecycle agent
# - Agent spawns worker
# - Annotates task: "Worker: impl-user-profile-clawd"
# - Sends Telegram notification

# Work in spawned worker session...
# Create PR, get it merged...

# Complete the task
task 1 done

# Hook automatically:
# - Checks Worker annotation
# - Checks PR status → merged
# - Calls ttal worker close --cleanup
# - Removes session, worktree, branch
# - Sends Telegram notification
```

### Troubleshooting

**Hook not triggering:**
```bash
# Check hook is executable
ls -l ~/.task/hooks/on-modify-worker-lifecycle
# Should show: -rwxr-xr-x

# Check hook detected
task diagnostics | grep hooks

# Check logs
tail ~/.task/hooks.log
```

**Worker not spawning:**
```bash
# Check agent exists
openclaw agents list

# Check Telegram chat ID in config
cat ~/.task/hooks/config.json

# Test agent manually
openclaw agent --message "Test" --agent worker-lifecycle --deliver
```

**Cleanup not working:**
```bash
# Check ttal is installed
which ttal

# List active workers
ttal worker list

# Manual cleanup
ttal worker close <session> --force
```

For detailed design and implementation: `~/clawd/docs/plans/2026-02-01-taskwarrior-hooks-design.md`

## Advanced Features (Future)

Additional taskwarrior features available:

- **Recurrence**: `task add "..." recur:weekly due:monday`
- **Contexts**: `task context define work project:work`
- **Custom reports**: Define in `~/.taskrc`
- **Sync**: Sync across devices with taskd server
- **UDA**: User Defined Attributes for custom metadata

See [Taskwarrior documentation](https://taskwarrior.org/docs/) for details.

## Troubleshooting

### Task command not found

```bash
# Check if taskwarrior is installed
which task

# Install if needed (macOS)
brew install task
```

### Python scripts not executable

```bash
chmod +x ~/clawd/skills/taskwarrior/scripts/*.py
```

### Cron jobs not triggering

```bash
# List configured cron jobs
openclaw cron list

# Check cron logs
openclaw gateway logs
```

## References

- [Taskwarrior Official Docs](https://taskwarrior.org/docs/)
- [Taskwarrior Best Practices](https://taskwarrior.org/docs/best-practices/)
- [Design Document](~/clawd/docs/plans/2026-01-31-taskwarrior-design.md)
- [OpenClaw Cron Documentation](https://docs.openclaw.ai/automation/cron-jobs)
