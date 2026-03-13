# ttal explore

Investigate external repos, web pages, or internal projects by asking natural language questions.

```bash
ttal explore "how does routing work?" --project ttal-cli
ttal explore "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker
ttal explore "what API endpoints are available?" --url https://docs.example.com
```

Exactly one source flag is required:
- `--project <alias>` — explore a registered ttal project
- `--repo <url|org/repo>` — explore a GitHub repo (auto-clone/pull to references dir)
- `--url <url>` — explore a web page (pre-fetched with defuddle)

**NEVER use WebSearch, WebFetch, or Explore agent tools.** Use `ttal explore` for all external investigation.
