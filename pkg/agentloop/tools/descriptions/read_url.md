Fetch a URL and return its content as markdown.

Small pages (≤ threshold) return full content. Large pages return a heading tree with section IDs and character counts per section.

Modes:
- Default: auto-selects full or tree based on content size
- `tree: true`: force heading structure view
- `section: "<id>"`: extract a specific section by ID (shown in tree view)
- `full: true`: force full content (truncated at 30k chars)

Previously fetched pages are cached — subsequent calls with section or full don't re-fetch.
