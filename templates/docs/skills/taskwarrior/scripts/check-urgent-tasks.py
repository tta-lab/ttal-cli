#!/usr/bin/env python3
"""
Smart task notification checker for taskwarrior.

Two-tier notification system:
- Tier 1: Urgent tasks (overdue, due soon, high-priority stale, unblocked)
- Tier 2: Suggestions (if nothing urgent, show interesting tasks)

Exit codes:
- 0: Clean or suggestions only
- 1: Urgent tasks exist
"""

import json
import subprocess
import sys
from datetime import datetime, timedelta
from pathlib import Path
import random


def run_task_command(filter_args):
    """Execute taskwarrior command and return JSON output."""
    try:
        result = subprocess.run(
            ["task"] + filter_args + ["export"],
            capture_output=True,
            text=True,
            check=True
        )
        if not result.stdout.strip():
            return []
        return json.loads(result.stdout)
    except subprocess.CalledProcessError:
        return []
    except json.JSONDecodeError:
        return []


def get_task_age_days(task):
    """Calculate days since task was created."""
    entry_date = datetime.fromisoformat(task["entry"].replace("Z", "+00:00"))
    return (datetime.now(entry_date.tzinfo) - entry_date).days


def format_task(task):
    """Format task for display."""
    task_id = task.get("id", "?")
    priority = task.get("priority", "")
    priority_str = f"[{priority}]" if priority else "[·]"
    description = task["description"]

    # Add due date if present
    if "due" in task:
        due_date = datetime.fromisoformat(task["due"].replace("Z", "+00:00"))
        today = datetime.now(due_date.tzinfo)
        days_diff = (due_date.date() - today.date()).days

        if days_diff < 0:
            due_str = f"(due {abs(days_diff)} days ago)"
        elif days_diff == 0:
            due_str = "(due today)"
        elif days_diff == 1:
            due_str = "(due tomorrow)"
        else:
            due_str = f"(due in {days_diff} days)"
        description += f" {due_str}"

    # Add age if task is old
    age_days = get_task_age_days(task)
    if age_days > 3:
        description += f" (added {age_days} days ago)"
    elif age_days == 1:
        description += " (added yesterday)"
    elif age_days == 0:
        description += " (added today)"

    return f"  #{task_id} {priority_str} {description}"


def get_urgent_tasks():
    """Find urgent tasks that need immediate attention."""
    urgent = {
        "overdue": [],
        "due_soon": [],
        "high_priority_stale": [],
        "blocked_ready": []
    }

    # Overdue tasks
    urgent["overdue"] = run_task_command([
        "status:pending",
        "due.before:today"
    ])

    # Due in next 3 days
    urgent["due_soon"] = run_task_command([
        "status:pending",
        "due.after:yesterday",
        "due.before:3days"
    ])

    # High-priority tasks older than 3 days
    all_high = run_task_command([
        "status:pending",
        "priority:H"
    ])
    urgent["high_priority_stale"] = [
        t for t in all_high if get_task_age_days(t) >= 3
    ]

    # Note: Blocked tasks with ready dependencies would require
    # parsing task dependencies, skipping for now

    return urgent


def get_suggestion_tasks():
    """Get interesting task suggestions using smart criteria."""
    suggestions = {
        "high_value": [],
        "quick_wins": [],
        "fresh_ideas": [],
        "random_pick": None
    }

    # High-value: High priority that aren't stale yet
    all_high = run_task_command(["status:pending", "priority:H"])
    suggestions["high_value"] = [
        t for t in all_high if get_task_age_days(t) < 3
    ][:2]  # Limit to 2

    # Quick wins: Medium/Low priority with no dependencies
    all_pending = run_task_command(["status:pending"])
    quick_wins = [
        t for t in all_pending
        if t.get("priority") in ["M", "L", None] and "depends" not in t
    ]
    suggestions["quick_wins"] = quick_wins[:2]  # Limit to 2

    # Fresh ideas: Recently added (last 48 hours)
    fresh = [t for t in all_pending if get_task_age_days(t) <= 2]
    suggestions["fresh_ideas"] = fresh[:2]  # Limit to 2

    # Random element: Pick one random pending task
    if all_pending:
        suggestions["random_pick"] = random.choice(all_pending)

    return suggestions


def print_urgent_report(urgent):
    """Print urgent tasks report."""
    has_urgent = any(urgent.values())

    if not has_urgent:
        return False

    print("🔴 Urgent Tasks Need Attention:\n")

    if urgent["overdue"]:
        print(f"OVERDUE ({len(urgent['overdue'])}):")
        for task in urgent["overdue"]:
            print(format_task(task))
        print()

    if urgent["due_soon"]:
        print(f"DUE SOON ({len(urgent['due_soon'])}):")
        for task in urgent["due_soon"]:
            print(format_task(task))
        print()

    if urgent["high_priority_stale"]:
        print(f"HIGH PRIORITY STALE ({len(urgent['high_priority_stale'])}):")
        for task in urgent["high_priority_stale"]:
            print(format_task(task))
        print()

    if urgent["blocked_ready"]:
        print(f"BLOCKED (DEPENDENCIES READY) ({len(urgent['blocked_ready'])}):")
        for task in urgent["blocked_ready"]:
            print(format_task(task))
        print()

    return True


def print_suggestion_report(suggestions, show_header=True):
    """Print suggestion tasks report."""
    # Deduplicate suggestions across categories
    seen_ids = set()
    unique_suggestions = []

    for category in ["high_value", "quick_wins", "fresh_ideas"]:
        for task in suggestions[category]:
            task_id = task.get("id")
            if task_id and task_id not in seen_ids:
                seen_ids.add(task_id)
                unique_suggestions.append((category, task))

    # Add random pick if not already included
    if suggestions["random_pick"]:
        task_id = suggestions["random_pick"].get("id")
        if task_id not in seen_ids:
            unique_suggestions.append(("random_pick", suggestions["random_pick"]))

    if not unique_suggestions:
        if show_header:
            print("✅ No tasks found!")
        return

    if show_header:
        print("✅ No urgent tasks!\n")

    print("💡 Suggested tasks to consider:\n")

    # Group by category
    categories = {
        "high_value": "HIGH VALUE",
        "quick_wins": "QUICK WINS",
        "fresh_ideas": "FRESH IDEAS",
        "random_pick": "RANDOM SUGGESTION"
    }

    for cat_key, cat_name in categories.items():
        cat_tasks = [t for c, t in unique_suggestions if c == cat_key]
        if cat_tasks:
            print(f"{cat_name}:")
            for task in cat_tasks:
                print(format_task(task))
            print()


def main():
    """Main entry point."""
    urgent = get_urgent_tasks()
    has_urgent = any(urgent.values())

    if has_urgent:
        # Show urgent tasks
        print_urgent_report(urgent)

        # Also show a few suggestions
        suggestions = get_suggestion_tasks()
        if any(suggestions.values()):
            print_suggestion_report(suggestions, show_header=False)

        sys.exit(1)  # Exit with error code for urgent tasks
    else:
        # Show suggestions only
        suggestions = get_suggestion_tasks()
        print_suggestion_report(suggestions, show_header=True)

        sys.exit(0)  # Exit clean


if __name__ == "__main__":
    main()
