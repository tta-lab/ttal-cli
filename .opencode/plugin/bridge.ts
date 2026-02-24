// bridge.ts: OpenCode plugin that bridges assistant output to the ttal daemon.
//
// For OpenCode agents running in tmux, this plugin replaces the JSONL watcher
// used by Claude Code agents. It pushes assistant text and context status to the
// daemon via its existing unix socket, which then forwards to Telegram.
//
// Env vars (set by ttal team start):
//   TTAL_AGENT_NAME  - agent identity for routing
//   TTAL_DATA_DIR    - data dir containing daemon.sock
//
// Symlink this into each OpenCode agent's workspace:
//   ln -s /path/to/ttal-cli/.opencode/plugin/bridge.ts \
//         /path/to/agent-workspace/.opencode/plugin/bridge.ts
//
// Note: named bridge.ts (not ttal-bridge.ts) to avoid .gitignore's ttal-* pattern.

import type { Plugin } from "@opencode-ai/plugin"
import { connect } from "node:net"

const AGENT_NAME = process.env.TTAL_AGENT_NAME || ""
const SOCKET_PATH = (() => {
  const dataDir = process.env.TTAL_DATA_DIR || `${process.env.HOME}/.ttal`
  return `${dataDir}/daemon.sock`
})()

function sendToDaemon(payload: object): Promise<void> {
  return new Promise<void>((resolve) => {
    const client = connect(SOCKET_PATH, () => {
      client.write(JSON.stringify(payload) + "\n")
      client.end()
      resolve()
    })
    client.on("error", () => resolve())
    // Timeout to avoid hanging if daemon is unresponsive
    client.setTimeout(3000, () => {
      client.destroy()
      resolve()
    })
  })
}

export const TtalBridge: Plugin = async () => {
  if (!AGENT_NAME) {
    // Not running as a ttal agent — plugin is a no-op.
    return {}
  }

  return {
    event: async ({ event }) => {
      // Bridge completed assistant text parts to Telegram via daemon.
      // Uses the same send protocol as `ttal send` (from-only = agent → Telegram).
      if (event.type === "message.part.updated") {
        const part = event.properties?.part
        if (part?.type === "text" && part?.time?.end) {
          await sendToDaemon({
            from: AGENT_NAME,
            message: part.text,
          })
        }
      }

      // Bridge context status when session goes idle (turn complete).
      if (event.type === "session.idle") {
        const props = event.properties
        if (props?.usage) {
          const u = props.usage
          const total = (u.input || 0) + (u.output || 0) + (u.reasoning || 0)
          const limit = u.limit || 200000
          await sendToDaemon({
            type: "statusUpdate",
            agent: AGENT_NAME,
            context_used_pct: (total / limit) * 100,
            context_remaining_pct: ((limit - total) / limit) * 100,
            model_id: props.model || "unknown",
            session_id: props.sessionID || "",
          })
        }
      }
    },
  }
}
