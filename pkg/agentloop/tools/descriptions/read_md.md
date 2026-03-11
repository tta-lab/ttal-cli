Read a markdown file with structure awareness.

Small files (≤ threshold) return full content. Large files return a heading tree with section IDs and character counts per section.

Modes:
- Default: auto-selects full or tree based on file size
- `tree: true`: force heading structure view
- `section: "<id>"`: extract a specific section by ID (shown in tree view)
- `full: true`: force full content (truncated at 30k chars)

Only files within allowed project directories can be read.
