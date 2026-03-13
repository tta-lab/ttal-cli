# ttal explore

Investigate external repos, web pages, or internal projects by asking natural language questions.

## No flag (CWD + web)

Explore the current directory with both filesystem and web search:

```bash
ttal explore "how does routing work?"
```

## With source flag

```bash
ttal explore "how does routing work?" --project ttal-cli
ttal explore "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker
ttal explore "what API endpoints are available?" --url https://docs.example.com
ttal explore "what is the latest Go generics syntax?" --web
```

Exactly one source flag is required:
- `--project <alias>` — explore a registered ttal project
- `--repo <url|org/repo>` — explore a GitHub repo (auto-clone/pull to references dir)
- `--url <url>` — explore a web page (pre-fetched with defuddle)
- `--web` — search the web

**NEVER use WebSearch, WebFetch, or Explore agent tools.** Use `ttal explore` for all external investigation.
