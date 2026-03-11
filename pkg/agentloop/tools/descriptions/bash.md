Execute a bash command in a sandboxed environment.

Available commands: python3, task, jq, curl, grep, sort, uniq, wc.
Piping works: `cat file.json | python3 -c "..."`.

- Read-only filesystem (no writes outside /tmp)
- Session-scoped /tmp directory
- 30-second timeout
- Network access enabled (DNS, HTTPS)

Do NOT use bash for file reading — use the read or read_md tools instead.
Do NOT use bash for file searching — use glob or grep instead.
