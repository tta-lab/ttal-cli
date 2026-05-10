---
name: ttal-messaging
description: Send messages to humans, agents, or workers through ttal send.
---

# ttal messaging

`ttal send --to <recipient>` is the same command for humans, agents, and workers. The recipient is a human alias, agent name, or worker address (`<job_id>:<agent_name>`).

## Send

```
cat <<'EOF' | ttal send --to <recipient>
message
EOF
```

Examples:

```
cat <<'EOF' | ttal send --to <human-alias>
done, PR ready
EOF

cat <<'EOF' | ttal send --to <agent-name>
can you review my auth module?
EOF

cat <<'EOF' | ttal send --to 1234abcd:coder
check the failing test in auth_test.go
EOF
```

## Worker Sessions

Worker sessions require explicit `job_id:agent_name` format. The daemon uses the job_id to find the tmux session and the agent_name as the window target. Workers construct their From address as `job_id:agent_name` so reply hints are always actionable.

## Stdin (preferred: heredoc)

```
cat <<'EOF' | ttal send --to kestrel
done
EOF
```

For longer messages, use the same heredoc form:

```
cat <<'ENDBASH' | ttal send --to kestrel
## Status Update
Auth module review complete. Two issues found:
1. Token expiry not checked
2. Missing rate limit
ENDBASH
```
