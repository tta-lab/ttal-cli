---
name: ttal-ask
description: Investigate external repos, web pages, or internal projects by asking natural language questions.
---

# ttal ask

Investigate external repos, web pages, or internal projects by asking natural language questions.

## No flag (CWD + web)

Ask about the current directory with both filesystem and web search:

```bash
ttal ask "how does routing work?"
```

## With source flag

```bash
ttal ask "how does routing work?" --project ttal-cli
ttal ask "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker
ttal ask "what API endpoints are available?" --url https://docs.example.com
ttal ask "what is the latest Go generics syntax?" --web
```

Use one source flag, or omit all flags for CWD mode:
- `--project <alias>` — ask about a registered ttal project
- `--repo <url|org/repo>` — ask about a GitHub repo (auto-clone/pull to references dir)
- `--url <url>` — ask about a web page (pre-fetched with defuddle)
- `--web` — search the web

**NEVER use WebSearch, WebFetch, or Explore agent tools.** Use `ttal ask` for all external investigation.
