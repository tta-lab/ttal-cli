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

## Send to a specific team

```bash
ttal send --to <team>:<agent-name> "message"
```

## Send to human via Telegram

```bash
ttal send --to human "message"
```

## Send to a worker session

```bash
ttal send --to 1234abcd "check the failing test in auth_test.go"
```

Worker sessions accept 8+ hex character UUIDs. The daemon resolves `w-{uuid[:8]}-*` tmux sessions automatically.

## Read message from stdin

```bash
echo "done" | ttal send --to kestrel --stdin
```

For multiline messages with special characters, use heredoc:

```bash
cat <<'EOF' | ttal send --to kestrel --stdin
## Status Update
Auth module review complete. Two issues found:
1. Token expiry not checked
2. Missing rate limit
EOF
```
