---
name: ttal-messaging
description: Send messages to agents or humans via Telegram.
---

# ttal messaging

Send messages to agents or humans.

## Send to another agent

```bash
cat <<'EOF' | ttal send --to <agent-name>
can you review my auth module?
EOF
```

## Send to a human via Telegram

```bash
cat <<'EOF' | ttal send --to <alias>
message
EOF
```

## Send to a worker session

```bash
cat <<'EOF' | ttal send --to 1234abcd:coder
check the failing test in auth_test.go
EOF
```

Worker sessions require explicit `job_id:agent_name` format. The daemon uses the job_id to find the tmux session and the agent_name as the window target. Workers construct their From address as `job_id:agent_name` so reply hints are always actionable.

## Stdin (preferred: heredoc)

```bash
cat <<'EOF' | ttal send --to kestrel
done
EOF
```

For longer messages, use the same heredoc form:

```bash
cat <<'ENDBASH' | ttal send --to kestrel
## Status Update
Auth module review complete. Two issues found:
1. Token expiry not checked
2. Missing rate limit
ENDBASH
```
