---
name: treemd
description: Use treemd to read long markdown documents efficiently. Scan structure with --tree, then extract specific sections with -s. Prefer this over reading entire files when docs are large.
---

# treemd — Structured Markdown Reading

Use `treemd` to read long markdown files (research docs, memory files, CLAUDE.md files) without loading the entire content into context.

## Two-Step Pattern

### 1. Scan structure

```bash
treemd --tree doc.md
```

Shows a collapsible heading tree with box-drawing:

```
└─ # Research Report
    ├─ ## Key Findings
    │   ├─ ### Architecture
    │   └─ ### Communication
    ├─ ## Recommendations
    └─ ## Sources
```

### 2. Extract the section you need

```bash
treemd -s "Exact Heading Text" doc.md
```

Returns only that section's content (including subsections).

**Important:** `-s` requires the **exact full heading text**. Get it from `--tree` or `-l` first.

## Flat heading list

```bash
treemd -l doc.md
```

Returns headings without tree formatting — useful for quick scanning.

## When to Use

- Reading research docs (`~/clawd/docs/research/`)
- Reading memory files (`memory/YYYY-MM-DD.md`)
- Reading any long markdown file where you only need specific sections
- Saves context window tokens compared to `Read` on a large file

## Limitations

- `--filter` and `-o json` require a TTY — they don't work in agent Bash (no terminal)
- `-s` needs exact heading match, no partial/fuzzy matching
- Ephemeral only — no persistent collapse state
