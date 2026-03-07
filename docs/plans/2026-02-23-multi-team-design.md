# Multi-Team Support Design

> Date: 2026-02-23

## Problem

ttal assumes a single team per machine. All paths (`~/.ttal/`, `~/.taskrc`, `~/.task/hooks/`)
are hardcoded. This prevents running multiple independent teams (e.g. Neil's personal team +
guion team with Sven) on the same Mac.

## Design

### Config Schema

`~/.config/ttal/config.toml` is team-aware. The `[teams]` section is **required** (enforced in code).

```toml
default_team = "personal"

[teams.personal]
data_dir = "~/.ttal"
taskrc = "~/.taskrc"
chat_id = "123456"
lifecycle_agent = "yuki"
voice_vocabulary = ["ttal", "treemd", "taskwarrior"]

[teams.personal.agents.kestrel]
bot_token = "bot456:DEF"
chat_id = "789012"

[teams.guion]
data_dir = "~/.ttal-guion"
taskrc = "~/.task-guion/taskrc"
chat_id = "111222"
lifecycle_agent = "athena"
voice_vocabulary = ["guion", "ttal"]

[teams.guion.agents.sven]
bot_token = "bot999:XYZ"
chat_id = "333444"
```

**Backward compatibility:** If no `[teams]` section exists, the flat config fields (`chat_id`,
`lifecycle_agent`, `agents`, `voice`) are treated as an implicit `"default"` team with
`data_dir = "~/.ttal"` and `taskrc = "~/.taskrc"`.

### Team Resolution

Active team is resolved in order:
1. `TTAL_TEAM` env var (set in tmux/launchd sessions)
2. `default_team` from config
3. `"default"` literal fallback

### Data Isolation

Each team gets its own data directory containing: database, daemon socket/pid, cleanup dir,
status files, git dumps, memory output, voice scripts. Two teams = two daemons, full isolation.

| Component | Path |
|-----------|------|
| Database | `<data_dir>/ttal.db` |
| Daemon socket | `<data_dir>/daemon.sock` |
| Daemon PID | `<data_dir>/daemon.pid` |
| Cleanup dir | `<data_dir>/cleanup/` |
| Status dir | `<data_dir>/status/` |
| Git dumps | `<data_dir>/dumps/` |
| Memory output | `<data_dir>/memory/` |
| Voice scripts | `<data_dir>/` |
| Daemon log | `<data_dir>/daemon.log` |

### Daemon Per Team

Each team runs its own daemon:
- Launchd plist: `io.guion.ttal.daemon.<team>.plist`
- Each plist bakes in `TTAL_TEAM=<team>`
- `ttal daemon install` uses the active team
- `ttal daemon status` shows the active team's daemon

### Env Propagation

When spawning tmux sessions (workers, agents), ttal sets:
- `TTAL_TEAM=<team>` — child processes resolve the same team
- `TASKRC=<taskrc>` — taskwarrior uses the team's instance
- `TTAL_AGENT_NAME=<agent>` — existing behavior, unchanged
- `TTAL_JOB_ID=<uuid[:8]>` — existing behavior, unchanged

### Taskwarrior Hook Installation

`ttal worker install` reads the active team's `taskrc` to determine the hooks directory
(parsed from `data.location` in the taskrc) instead of hardcoding `~/.task/hooks/`.

## Implementation Scope

### 1. Config layer (`internal/config/`)
- Add `TeamConfig` struct with `DataDir`, `TaskRC`, `ChatID`, `LifecycleAgent`,
  `VoiceVocabulary`, `Agents`
- Add `DefaultTeam` field to top-level `Config`
- Add `Teams map[string]TeamConfig` to `Config`
- Add `ActiveTeam()`, `DataDir()`, `TaskRC()` resolution functions
- Backward compat: promote flat config to implicit `"default"` team

### 2. Path helpers (~10 files)
Replace `filepath.Join(home, ".ttal", ...)` with `config.DataDir()`:
- `internal/db/db.go`
- `internal/daemon/daemon.go`, `socket.go`, `launchd.go`
- `internal/worker/cleanup.go`, `hook.go`, `hook_enrich.go`
- `internal/status/status.go`
- `internal/gitutil/gitutil.go`
- `cmd/memory.go`
- `internal/voice/install.go`

### 3. Env propagation
Set `TTAL_TEAM` + `TASKRC` when spawning:
- `internal/worker/spawn.go`
- `internal/team/start.go`
- `internal/daemon/launchd.go`

### 4. Taskwarrior hooks (`internal/worker/install.go`)
- Read `TaskRC()` to find the hooks directory

### 5. Doctor (`internal/doctor/`)
- Check the active team's taskrc path instead of hardcoding `~/.taskrc`

### 6. Daemon naming (`internal/daemon/launchd.go`)
- Plist label includes team name: `io.guion.ttal.daemon.<team>`

## Backward Compatibility

- **No config migration required.** Flat configs work as implicit `"default"` team.
- **No data migration.** Existing `~/.ttal/` stays. New teams get fresh data dirs.
- **Existing daemon plist** works until user opts into teams, then re-runs `ttal daemon install`.
- **Existing tmux sessions** without `TTAL_TEAM` resolve to default team.
