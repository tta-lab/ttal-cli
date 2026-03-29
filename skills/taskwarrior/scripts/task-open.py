#!/usr/bin/env python3
"""
Display taskwarrior task with inline markdown documentation.

Fetches task details and renders any referenced markdown files from annotations.

Usage:
    task-open.py <task_id>
"""

import json
import subprocess
import sys
from pathlib import Path
import re


def run_task_export(task_id):
    """Export task data as JSON."""
    try:
        result = subprocess.run(
            ["task", str(task_id), "export"],
            capture_output=True,
            text=True,
            check=True
        )
        if not result.stdout.strip():
            return None
        tasks = json.loads(result.stdout)
        return tasks[0] if tasks else None
    except subprocess.CalledProcessError as e:
        print(f"Error: Failed to export task {task_id}", file=sys.stderr)
        print(f"  {e.stderr}", file=sys.stderr)
        return None
    except json.JSONDecodeError:
        print(f"Error: Invalid JSON from task export", file=sys.stderr)
        return None


def extract_file_references(annotations):
    """Extract file paths from annotations.

    Recognizes patterns like:
    - Design: ~/path/to/file.md
    - Doc: ~/path/to/file.md
    - Reference: ~/path/to/file.md
    - File: ~/path/to/file.md
    """
    if not annotations:
        return []

    file_refs = []
    pattern = r'(?:Design|Doc|Reference|File):\s*([~\/][\w\/\-\.]+\.md)'

    for annotation in annotations:
        description = annotation.get("description", "")
        matches = re.findall(pattern, description)
        for match in matches:
            # Expand ~ to home directory
            file_path = Path(match).expanduser()
            file_refs.append((annotation.get("description"), file_path))

    return file_refs


def format_task_header(task):
    """Format task metadata as header."""
    lines = []

    # Task ID and description
    task_id = task.get("id", "?")
    description = task["description"]
    lines.append(f"Task #{task_id}: {description}")

    # Project
    if "project" in task:
        lines.append(f"Project: {task['project']}")

    # Tags
    if "tags" in task:
        tags = ", ".join(task["tags"])
        lines.append(f"Tags: {tags}")

    # Priority
    if "priority" in task:
        lines.append(f"Priority: {task['priority']}")

    # Due date
    if "due" in task:
        due = task["due"].replace("Z", "").replace("T", " ")[:16]
        lines.append(f"Due: {due}")

    # Status
    status = task.get("status", "unknown")
    lines.append(f"Status: {status.title()}")

    # Annotations (non-file-reference ones)
    file_refs = extract_file_references(task.get("annotations", []))
    file_ref_descriptions = {ref[0] for ref in file_refs}

    other_annotations = [
        ann for ann in task.get("annotations", [])
        if ann.get("description") not in file_ref_descriptions
    ]

    if other_annotations:
        lines.append("\nAnnotations:")
        for ann in other_annotations:
            desc = ann.get("description", "")
            # Handle multiline annotations
            for line in desc.split("\n"):
                lines.append(f"  {line}")

    return "\n".join(lines)


def print_separator(char="═", width=80):
    """Print a separator line."""
    print(char * width)


def print_markdown_file(label, file_path):
    """Read and print markdown file content."""
    print_separator()
    print(f"{label}")
    print_separator("─")

    try:
        content = file_path.read_text()
        print(content)
    except FileNotFoundError:
        print(f"[File not found: {file_path}]")
    except PermissionError:
        print(f"[Permission denied: {file_path}]")
    except Exception as e:
        print(f"[Error reading file: {e}]")

    print_separator()


def main():
    """Main entry point."""
    if len(sys.argv) != 2:
        print("Usage: task-open.py <task_id>", file=sys.stderr)
        sys.exit(1)

    task_id = sys.argv[1]

    # Fetch task data
    task = run_task_export(task_id)
    if not task:
        print(f"Error: Task {task_id} not found", file=sys.stderr)
        sys.exit(1)

    # Print task header
    print(format_task_header(task))
    print()

    # Extract and display file references
    file_refs = extract_file_references(task.get("annotations", []))

    if file_refs:
        print("Referenced Documentation:")
        for label, file_path in file_refs:
            print_markdown_file(label, file_path)
    else:
        # No file references found
        pass


if __name__ == "__main__":
    main()
