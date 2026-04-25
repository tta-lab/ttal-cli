---
name: ttal-messaging
description: Send messages to agents or humans via Telegram.
---

# ttal messaging

Send messages to agents or humans.

## Send to another agent

```bash
ttal send --to <agent-name> "can you review my auth module?"
```

## Send to a human via Telegram

```bash
ttal send --to <alias> "message"
```

## Send to a worker session

```bash
ttal send --to 1234abcd:coder "check the failing test in auth_test.go"
```

Worker sessions require explicit `job_id:agent_name` format. The daemon uses the job_id to find the tmux session and the agent_name as the window target. Workers construct their From address as `job_id:agent_name` so reply hints are always actionable.

## Piped stdin (auto-detected, no flag needed)

```bash
echo "done" | ttal send --to kestrel
```

For multiline messages with special characters, use heredoc:

```bash
cat <<'ENDBASH' | ttal send --to kestrel
## Status Update
Auth module review complete. Two issues found:
1. Token expiry not checked
2. Missing rate limit
ENDBASH
```
