# Containers

Manager plane container image built with the [Dagger Go SDK](https://docs.dagger.io/sdk/go).

## Images

| Image | Description |
|-------|-------------|
| `ttal-manager-cc` | Manager plane with Claude Code |

Future: `ttal-manager-opencode` for OpenCode-based agents.

## Build

```bash
cd containers/manager/

# Export to local OCI tarball
dagger run go run main.go

# Push to GHCR
dagger run go run main.go --push --tag latest
dagger run go run main.go --push --tag v0.1.0
```

## Pull from GHCR

```bash
podman pull ghcr.io/tta-lab/ttal-manager-cc:latest
```

## What's Baked In vs Mounted

**Baked into image:**
- Node.js 22 (base image)
- Claude Code (`@anthropic-ai/claude-code` via npm)
- taskwarrior, tmux, git, fish, curl, wget, jq, tree

**Mounted at runtime:**
- `ttal`, `flicknote`, `diary` — local dev binaries, vary per machine
- All credentials and config — never baked into images

## Runtime Mounts

### Binaries (read-only)

| Binary | Container Path |
|--------|----------------|
| `ttal` | `/usr/local/bin/ttal` |
| `flicknote` | `/usr/local/bin/flicknote` |
| `diary` | `/usr/local/bin/diary` |

### Volumes

| Volume | Container Path | Mode | Purpose |
|--------|----------------|------|---------|
| `~/.claude/` | `/home/node/.claude/` | `rw` | Credentials, skills, JSONL logs |
| `~/.config/ttal/` | `/home/node/.config/ttal/` | `ro` | config.toml, .env, roles.toml |
| `~/.ssh/` | `/home/node/.ssh/` | `ro` | Git over SSH |
| `~/.taskrc` | `/home/node/.taskrc` | `ro` | Taskwarrior config |
| `~/.task/` | `/home/node/.task/` | `rw` | Task database + hooks |
| Agent workspace | `/workspace/` | `rw` | Memory, CLAUDE.md |

## Example: podman run

```bash
podman run -it --rm \
  -v $(which ttal):/usr/local/bin/ttal:ro \
  -v $(which flicknote):/usr/local/bin/flicknote:ro \
  -v $(which diary):/usr/local/bin/diary:ro \
  -v ~/.claude:/home/node/.claude:rw \
  -v ~/.config/ttal:/home/node/.config/ttal:ro \
  -v ~/.ssh:/home/node/.ssh:ro \
  -v ~/.taskrc:/home/node/.taskrc:ro \
  -v ~/.task:/home/node/.task:rw \
  -v ~/clawd:/workspace:rw \
  ghcr.io/tta-lab/ttal-manager-cc:latest \
  fish
```
