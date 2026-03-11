Find files matching a glob pattern within allowed project directories.

Supports `**` for recursive matching (e.g., `**/*.go`, `src/**/*.ts`).

- Returns up to 200 results sorted by modification time (most recent first)
- When path is empty, searches all allowed directories
- Path must be within allowed directories

Use this to discover files before reading them with read or read_md.
