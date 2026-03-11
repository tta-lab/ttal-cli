Read a file and return its contents with line numbers.

Only files within allowed project directories can be read. Use glob to discover files first.

- Default: 2000 lines from start
- Use offset/limit for large files
- Max file size: 5MB
- Lines longer than 2000 chars are truncated

For markdown files (.md), prefer read_md which provides structure-aware reading with tree view and section extraction.
