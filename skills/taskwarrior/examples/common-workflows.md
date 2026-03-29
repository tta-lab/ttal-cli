# Common Taskwarrior Workflows

Practical examples of everyday task management patterns.

## Quick Reference

```bash
# Create task
task add "Description" project:name +tag priority:H due:tomorrow

# List tasks
task                          # All pending
task project:clawd            # By project
task +bugfix                  # By tag
task priority:H               # By priority

# Manage task
task 1 start                  # Mark as active
task 1 done                   # Complete
task 1 delete                 # Remove
task 1 annotate "Note"        # Add note
task 1 modify due:tomorrow    # Update

# View details
task 1                        # Basic info
~/clawd/skills/taskwarrior/scripts/task-open.py 1  # With markdown context

# Reports
task summary                  # By project
task stats                    # Statistics
task burndown.weekly          # Burndown chart
```

## Scenario: Bug Fix

**Situation:** User reports a bug that needs immediate attention.

```bash
# 1. Create high-priority bug task
task add "Fix authentication timeout on large datasets" \
  project:guion.api \
  +bugfix \
  priority:H \
  due:tomorrow

# Taskwarrior assigns ID (e.g., #42)

# 2. Add investigation notes
task 42 annotate "User report: Timeouts after 30 seconds on datasets >1000 rows
Initial hypothesis: N+1 query problem in user relationships
Check: Database slow query log, API profiler output"

# 3. Start working on it
task 42 start

# 4. Add findings as you investigate
task 42 annotate "Confirmed: N+1 queries in getUserRelationships()
Solution: Add eager loading with JOIN"

# 5. Complete when fixed
task 42 done
```

## Scenario: New Feature Implementation

**Situation:** Planning and implementing a complex new feature.

```bash
# 1. Create design task with external documentation
task add "Design real-time notification system" \
  project:guion \
  +design \
  priority:H \
  due:3days

# Taskwarrior assigns ID (e.g., #15)

# 2. Link to detailed design document
task 15 annotate "Design: ~/clawd/docs/plans/2026-01-31-notification-system.md"

# 3. Create the design document
# (Write detailed architecture, tech choices, etc. in markdown file)

# 4. Review task with full context
~/clawd/skills/taskwarrior/scripts/task-open.py 15

# 5. Start implementation task after design approved
task add "Implement real-time notification system" \
  project:guion \
  +implementation \
  priority:H \
  due:1week

task 16 annotate "Reference: Design in task #15"
task 16 start

# 6. Complete both tasks when done
task 15 done
task 16 done
```

## Scenario: Research Task

**Situation:** Need to research a technology before making a decision.

```bash
# 1. Create research task
task add "Research Effect.ts error handling patterns" \
  project:clawd \
  +research \
  priority:M

# 2. Add research questions
task 17 annotate "Questions to answer:
- How does Effect handle async errors?
- Best practices for error composition
- Comparison with traditional try/catch
- Integration with existing codebase"

# 3. Start research
task 17 start

# 4. Add findings
task 17 annotate "Findings: Effect uses Either type for expected errors
Can compose errors elegantly with catchAll/catchTag
Recommendation: Adopt for new async operations"

# 5. Create follow-up implementation task
task add "Refactor API error handling to use Effect.ts" \
  project:clawd \
  +refactor \
  priority:M \
  due:1week

task 18 annotate "Based on research in task #17"

# 6. Complete research task
task 17 done
```

## Scenario: Weekly Sprint Planning

**Situation:** Start of week, planning what to work on.

```bash
# 1. Review all pending tasks
task

# 2. Check project summary
task summary

# 3. Review overdue and due soon
task due.before:3days

# 4. Pick high-priority tasks for the week
task project:clawd priority:H

# 5. Set due dates for weekly goals
task 12 modify due:friday
task 15 modify due:friday
task 18 modify due:friday

# 6. Start first task
task 12 start
```

## Scenario: Daily Standup

**Situation:** Preparing for daily standup meeting.

```bash
# 1. What I completed yesterday
task end.after:yesterday completed

# 2. What I'm working on today (active tasks)
task status:active

# 3. What I plan to do today (pending high priority)
task status:pending priority:H

# 4. Any blockers (would need dependencies feature)
# (Future: task status:pending +blocked)
```

## Scenario: End of Week Review

**Situation:** Friday afternoon, reviewing the week's progress.

```bash
# 1. See what was completed this week
task end.after:today-7days completed

# 2. Check burndown for the week
task burndown.weekly

# 3. Review what's still pending
task status:pending

# 4. Defer low-priority tasks to next week
task +lowpriority modify wait:monday

# 5. Update due dates for incomplete tasks
task 25 modify due:monday
```

## Scenario: Context Switching

**Situation:** Need to switch between different projects.

```bash
# 1. See all tasks for specific project
task project:clawd

# 2. Filter by project and priority
task project:clawd priority:H

# 3. Combine project and tags
task project:guion +bugfix

# 4. Start task from different project
task 30 start

# (Future: Use taskwarrior contexts for automatic filtering)
# task context define work project:guion
# task context work
```

## Scenario: Managing Interruptions

**Situation:** Working on task A, urgent task B comes in.

```bash
# Current: Working on task 10
task 10 status:active

# Urgent task arrives
task add "Critical: Production database migration failing" \
  project:guion \
  +bugfix +urgent \
  priority:H

# Start urgent task (task 10 automatically stops being active)
task 50 start

# Complete urgent task
task 50 done

# Resume original task
task 10 start
```

## Scenario: Batch Task Creation

**Situation:** Breaking down large task into subtasks.

```bash
# Main task
task add "Migrate authentication to OAuth 2.1" \
  project:guion \
  +refactor \
  priority:H

# Subtasks (no native subtask support, use project hierarchy)
task add "Update OAuth library to v2.1" \
  project:guion.auth \
  +refactor \
  priority:H

task add "Implement PKCE flow" \
  project:guion.auth \
  +implementation \
  priority:H

task add "Update refresh token rotation" \
  project:guion.auth \
  +implementation \
  priority:H

task add "Migrate existing user sessions" \
  project:guion.auth \
  +migration \
  priority:H

# Track in annotations
task 51 annotate "Subtasks: #52, #53, #54, #55"
```

## Scenario: Using File References

**Situation:** Task requires detailed documentation.

```bash
# 1. Create task with doc reference
task add "Implement caching layer" \
  project:guion \
  +implementation \
  priority:H

task 60 annotate "Design: ~/clawd/docs/plans/2026-01-31-caching-design.md"

# 2. Create additional reference docs
task 60 annotate "Reference: ~/clawd/docs/architecture/caching-strategy.md"

# 3. View task with all documentation
~/clawd/skills/taskwarrior/scripts/task-open.py 60

# The script will display:
# - Task metadata
# - All annotations
# - Full content of both markdown files
```

## Scenario: Manual Notification Check

**Situation:** Want to check notifications outside scheduled times.

```bash
# Run notification script manually
~/clawd/skills/taskwarrior/scripts/check-urgent-tasks.py

# Check exit code
echo $?
# 0 = clean or suggestions only
# 1 = urgent tasks exist

# Use in scripts
if ~/clawd/skills/taskwarrior/scripts/check-urgent-tasks.py; then
  echo "All good!"
else
  echo "Urgent tasks need attention!"
fi
```

## Scenario: Cleaning Up Old Tasks

**Situation:** Removing tasks that are no longer relevant.

```bash
# 1. Review old pending tasks (created >30 days ago)
task entry.before:today-30days status:pending

# 2. Delete tasks that are no longer relevant
task 70 delete

# 3. Mark as waiting if might be relevant later
task 71 modify wait:1month

# 4. Review waiting tasks
task status:waiting
```

## Advanced Filtering

```bash
# Combine multiple filters
task project:guion priority:H due.before:3days

# Use OR logic with parentheses
task '(project:clawd or project:guion)' priority:H

# Exclude with -
task project:guion -bugfix

# Date ranges
task due.after:today due.before:friday

# Tag combinations
task +bugfix +urgent

# Complex queries
task '(priority:H or due.before:tomorrow)' status:pending -waiting
```

## Tips and Tricks

### Bulk Operations

```bash
# Modify multiple tasks at once
task +old-tag modify +new-tag
task project:old modify project:new
task priority:L modify priority:M
```

### Counting Tasks

```bash
# Count tasks matching criteria
task +bugfix count
task project:clawd status:pending count
```

### Sorting and Limiting

```bash
# Sort by urgency (taskwarrior's calculated field)
task list urgency

# Show only top 5 most urgent
task limit:5 urgency
```

### Exporting Data

```bash
# Export all tasks as JSON
task export > tasks-backup.json

# Export specific project
task project:clawd export > clawd-tasks.json
```

## Integration Examples

### Git Commit Workflow

```bash
# Reference task in commit message
git commit -m "fix(auth): resolve timeout issue

Fixes task #42: Authentication timeout on large datasets
Added eager loading to prevent N+1 queries"

# Update task when PR merged
task 42 annotate "Fixed in PR #234, merged to main"
task 42 done
```

### Note-Taking Integration

```bash
# Link to external notes
task 80 annotate "Notes: ~/Documents/meeting-notes-2026-01-31.md"

# View with task-open.py
~/clawd/skills/taskwarrior/scripts/task-open.py 80
```

### Calendar Integration

```bash
# Tasks with due dates can sync to calendar
# (Requires third-party tools like taskwarrior-calendar-sync)

# For now, manually check calendar view
task calendar
```

## Common Mistakes to Avoid

### Over-using Due Dates

❌ Bad:
```bash
task add "Learn new framework" due:tomorrow
task add "Read documentation" due:tomorrow
task add "Write example code" due:tomorrow
```

✅ Good:
```bash
task add "Learn new framework" priority:M
task add "Implement feature X with new framework" due:friday priority:H
```

**Why:** Due dates should be for time-sensitive tasks. Over-use leads to "due date fatigue".

### Vague Descriptions

❌ Bad:
```bash
task add "Fix bug"
task add "Update code"
task add "Check thing"
```

✅ Good:
```bash
task add "Fix authentication timeout on large datasets" +bugfix
task add "Update OAuth library to v2.1" +upgrade
task add "Check database query performance" +investigation
```

### Putting Everything in Annotations

❌ Bad:
```bash
task add "Task" project:x
task 1 annotate "This task is about implementing the new authentication system which will replace the old one. The new system will use OAuth 2.1 instead of OAuth 2.0. We need to update the library first, then implement PKCE flow, then migrate existing sessions. The deadline is next Friday because the client needs it urgently. Also need to update documentation and write tests."
```

✅ Good:
```bash
task add "Migrate authentication to OAuth 2.1" project:x priority:H due:friday
task 1 annotate "Design: ~/clawd/docs/plans/auth-migration.md"
# Put detailed planning in the markdown file
```

### Not Using Projects

❌ Bad:
```bash
task add "Fix login bug" +bug
task add "Implement user profile" +feature
task add "Update database schema" +refactor
# All tasks mixed together
```

✅ Good:
```bash
task add "Fix login bug" project:guion.auth +bug
task add "Implement user profile" project:guion.user +feature
task add "Update database schema" project:guion.db +refactor
# Organized by project
```

## See Also

- [SKILL.md](../SKILL.md) - Complete skill documentation
- [Taskwarrior Docs](https://taskwarrior.org/docs/) - Official documentation
- [Design Document](~/clawd/docs/plans/2026-01-31-taskwarrior-design.md) - Architecture details
