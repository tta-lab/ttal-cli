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

## Read message from stdin

```bash
echo "done" | ttal send --to kestrel --stdin
```
