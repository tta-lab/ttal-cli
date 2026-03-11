Search file contents using a regex pattern within allowed project directories.

Returns matching lines with file paths and line numbers. Output capped at 30,000 characters.

- Supports full regex syntax
- Optional glob filter to narrow file types (e.g., `*.go`)
- When path is empty, searches all allowed directories
- Path must be within allowed directories

Use glob to find files by name, grep to find files by content.
