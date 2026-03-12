- don't add claude.ai link in commit message
- for cloudflare worker, we are using wrangler.jsonc, not wrangler.toml
- **Always use hex UUID (e.g., 1234abcd) when referencing tasks** — numeric IDs shift when tasks complete/delete

## ttal Two-Plane Architecture

**Manager Plane** — Long-running agents (orchestrator, researcher, designer). Runs on Claude Code / OpenCode / Codex CLI. Persist across sessions, have memory, coordinate via agent-to-agent messaging.

**Worker Plane** — Short-lived coders/reviewers. Spawned on demand per task, isolated in git worktrees within tmux sessions. Run in parallel, implement → review → merge → done.

## Workflow & Planning

**Don't use plan mode for planning tasks** - Use brainstorming skill or writeplan skill instead

## Delegating Coding Work

**Always delegate coding to workers — don't implement yourself.**

### Step 1: Create the task
```bash
ttal task add --project <alias> "description"
```

### Step 2: Document context (choose by task size)

**Small task** — annotate inline:
```bash
ttal task add --project <alias> "description" --annotate "specific details, edge cases, approach"
```

**Large task** — write a plan in flicknote, then annotate with the note ID:
```bash
# Project name MUST contain "plan" or "fix" so workers can find it
flicknote add "# Plan: ..." --project myproject-plan
# or
flicknote add "# Fix: ..." --project myproject-fix

# Then annotate the task with the flicknote ID
ttal task add --project <alias> "description" --annotate "plan: flicknote/<id>"
```

Workers automatically look for flicknote notes in projects named `*plan*` or `*fix*`. Using this naming convention ensures workers see your context without you needing to paste it manually.

### Step 3: Execute
```bash
ttal task execute <uuid>    # spawns a worker in isolated worktree
```

## GitHub & Forgejo

- **Use `ttal pr` for all PR operations** — creation, modification, merging, commenting. Never use `gh`, `tea`, `curl`, or Forgejo MCP for PR work.
  - `ttal pr create "title" --body "description"` / `ttal pr modify --title "new" --body "new desc"` / `ttal pr merge` / `ttal pr comment create "msg"`

## Git Best Practices

- Always describe what's in git diff --cached, not your editing journey.

  Before committing:

  1. Run git diff --cached to see actual changes
  2. Write message based on the diff, not the process
  3. Ignore edits you made and reverted

  ❌ Wrong: "Removed logging" (if you added then removed it during editing)
  ✅ Right: "Add error handling" (what the diff actually shows)

  The commit message documents what changed between commits, not how you got there.

- never use bitnami images/helm charts, they are archived/deprecated
- we need to always use feat(something): fix(something): refactor(something): chore(something): syntax for git commits
- if possible, use Guard statement to reduce cyclomatic complexity
- for workers project, you should never run `bun run deploy` as we deploy by `git push`
- use these when you write package.json for a @flicknote package to make sure the github action works: "@semantic-release/git": "^10.0.1",
  "@semantic-release/npm": "^13.1.2",
  "semantic-release": "^25.0.2",
- before publish a npm package via push, remember to run npm i to make sure package-lock is correct.
- you should use bun install for non-npm-publishable-package proj
- don't create re-export files for backward compatibility - just update imports directly
- when moon typecheck shows cached results, trust them - don't try alternative methods like `bunx tsc` or `bun run typecheck`
- when adding new dependencies, run `bun install <package>` in root to get latest version - don't manually write potentially outdated versions in package.json
- **NEVER use WebSearch, WebFetch, or Explore agent tools.** Use `ttal explore` instead — it handles repos, web pages, and projects in one command:
  - `ttal explore "question" --repo org/repo` — explore OSS repos (auto-clone/pull)
  - `ttal explore "question" --url https://example.com` — explore web pages (pre-fetched with defuddle)
  - `ttal explore "question" --project <alias>` — explore registered ttal projects

## ttal CLI

ttal is the central CLI for agent coordination. Key commands:

### Agents & Team
```bash
ttal agent info <name>    # look up agent role, tools, personality
ttal agent list           # see all agents with emojis
```
ttal is the SSOT for agent identity. Don't hardcode agent info — query ttal.

### Messaging
Messages arrive as prefixed text in your input:
- `[telegram from:<name>]` — from a human via Telegram
- `[agent from:<name>]` — from another agent

**Replying to humans (Telegram):** Just output text naturally. The bridge picks up your response and delivers it to Telegram automatically. Don't use `ttal send` for this.

**Sending to another agent:**
```bash
ttal send --to <agent-name> "message"
```

**When to reply:**
- Meaningful updates: task complete, blocked, need input, PR ready
- Keep replies concise
- You don't need to reply to every message — use judgement

### Tasks & Today
```bash
ttal today list              # show today's focus tasks
ttal today add <uuid>...     # add tasks to today
ttal today completed         # what got done today
ttal task find <keywords>    # find tasks by keyword (OR match)
ttal task get <uuid>         # get formatted task prompt
```

### Task Routing

Route tasks to the right agent instead of doing everything yourself.

```bash
ttal task route <uuid> --to <agent>    # route to agent for design/research/brainstorm
ttal task execute <uuid>               # spawn a worker to implement the task
```

**When to use:**
- `ttal task route` — task needs design, research, or brainstorming. Use `/task-route` command to classify readiness and pick the right agent.
- `ttal task execute` — task has a plan/design doc annotated and is ready to implement. Spawns a Claude Code worker in its own tmux session + git worktree.

### Explore

Investigate external repos, web pages, or internal projects:

```bash
ttal explore "how does routing work?" --project ttal-cli
ttal explore "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker
ttal explore "what API endpoints are available?" --url https://docs.example.com
```

### Projects
```bash
ttal project list            # list all projects
ttal project get <name>      # project details (path, tags, etc.)
```

### Voice
```bash
ttal voice speak "text"      # TTS → Telegram voice message
ttal voice speak "text" --voice af_heart  # specific voice
```

### Sync (deploy skills & subagents)
```bash
ttal sync                    # deploy skills + subagents to runtime dirs
ttal sync --dry-run          # preview what would be deployed
ttal sync --clean            # remove stale deployments
```
Sources: `/Users/neil/Code/guion-opensource/ttal-cli/templates/docs/skills/` → `~/.claude/skills/`, `/Users/neil/Code/guion-opensource/ttal-cli/templates/docs/agents/` → `~/.claude/agents/`

## Learning & Knowledge

- `/Users/neil/Code/guion-opensource/ttal-cli/templates/docs/learning/` is where all agent learning notes go (book notes, insights, patterns discovered)
- Use the knowledge skill for folder routing and frontmatter conventions

## Git Committing Scope

- **Commit freely across the repo** — all workers use isolated worktrees, so there's no risk of stepping on others' work. If you see uncommitted files from other agents on `main`, commit them.

## Aliases
ef = effect.TS
ff = fast-forward
con = continue
ccon = commit and continue
cap = commit and push
cnp = commit but not push
c7 = use context7 to read docs
yr = use your recommendation
ka = keep it as-is
ssot = single source of truth
cpr = create pr
anno = annotate (task annotation)
post = post updates with `ttal pr comment create`


<!-- ttal-rules-start -->
## Shared Knowledge

### ttal-cli

# ttal Quick Reference

## Messaging

```bash
ttal send --to <agent> "message"
```

## Tasks

```bash
ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"
ttal task get <uuid>                    # rich prompt with inlined docs
ttal task find <keyword>                # search pending tasks
ttal task find <keyword> --completed    # search completed tasks
ttal task route <uuid> --to <agent>    # route to a specific agent
ttal task execute <uuid>               # spawn worker
```

## PRs

```bash
ttal pr create "title" --body "description"
ttal pr modify --title "new" --body "new"
ttal pr merge                          # squash merge
ttal pr comment create "markdown"
ttal pr comment list
```

For multiline comments with special characters, use heredoc:

```bash
cat <<'EOF' | ttal pr comment create
## Review
Changes look good. The fix correctly moves OPENCODE_PERMISSION from tmux session environment to buildEnvParts.

**LGTM**
EOF
```

## Projects & Agents

```bash
ttal project list                      # all active projects
ttal project get <alias>               # project details
ttal agent info <name>                 # agent details
ttal agent list                        # all agents
```

## Today

```bash
ttal today list                        # tasks scheduled today
ttal today add <uuid>                  # schedule for today
ttal today completed                   # done today
```

## Voice

```bash
ttal voice speak "text"                # speak with your voice
ttal voice speak "text" --voice <id>   # specific voice
ttal voice status                      # check server
```

### flicknote-cli

# flicknote Quick Reference

## Add

```bash
flicknote add "content" --project <name>
flicknote add "https://example.com"           # auto-detected as link
```

For multiline content with special characters, use heredoc:

```bash
cat <<'EOF' | flicknote add --project <name>
# My Note
Some content with **markdown** and $variables
EOF
```

## List

```bash
flicknote list --project <name>
flicknote list --search "keyword"
flicknote list --json
```

## Read

```bash
flicknote get <id>
flicknote get <id> --tree                     # heading structure
flicknote get <id> --section "Section Name"
flicknote get <id> --json
```

## Replace / Append

```bash
echo "new content" | flicknote replace <id>
echo "new content" | flicknote replace <id> --section "Name"
echo "more content" | flicknote append <id>
```

For multiline content with special characters, use heredoc:

```bash
cat <<'EOF' | flicknote replace <id>
# Updated Note
Content with **markdown** and $variables
EOF
```

## Section Operations

```bash
flicknote remove <id> --section "Name"
flicknote rename <id> --section "Old" "New"
echo "content" | flicknote insert <id> --before "Section"
echo "content" | flicknote insert <id> --after "Section"
```

## Archive

```bash
flicknote archive <id>
```

Never pipe flicknote content through sed/awk — use replace/insert instead.

<!-- ttal-rules-end -->
