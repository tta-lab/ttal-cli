# Telegram Library Decision

## Context

The ttal daemon needs to send and receive Telegram messages for bidirectional agent communication.
We evaluated four Go libraries before settling on one.

## Candidates

### 1. `go-telegram-bot-api/telegram-bot-api` (v5)
- **Stars:** ~6.4k — most popular Go bot library
- **API:** Bot HTTP API
- **Verdict:** Rejected — unmaintained since 2022, stuck on Bot API v5. No active releases.

### 2. `gotd/td`
- **Stars:** ~2.1k
- **API:** MTProto (raw Telegram protocol, not the Bot API)
- **Verdict:** Rejected — MTProto is for building full Telegram clients (user accounts, channels, media). Requires `appID` + `appHash` from Telegram's developer portal, not just a bot token. Massively over-engineered for a notification bot.

### 3. `mymmrac/telego`
- **Stars:** ~947
- **API:** Bot HTTP API
- **Verdict:** Rejected — more complex than needed (handler routing system on top of the core), smaller community. Used by picoclaw but they appear to be evaluating alternatives.

### 4. `go-telegram/bot`
- **Stars:** ~1.6k
- **API:** Bot HTTP API v9.3 (December 2025)
- **Verdict:** ✅ **Chosen**

## Why `go-telegram/bot`

1. **Zero dependencies** — nothing to audit transitively; reduces supply chain risk
2. **Actively maintained** — tracks latest Bot API (v9.3 as of Dec 2025)
3. **Context-first API** — `b.Start(ctx)` shuts down cleanly when context is cancelled, which integrates naturally with our signal handler and `done` channel pattern
4. **Right level of abstraction** — provides `getUpdates` long-polling and `sendMessage` without ceremony; we don't need telego's full handler routing
5. **Listed on official Telegram bot library page** (`core.telegram.org/bots/samples#go`)

## Usage in ttal

- `sendTelegramMessage()` — fire-and-forget outbound notifications via `b.SendMessage()`
- `startTelegramPoller()` — long-poll loop per agent bot token; delivers inbound messages to zellij via `write-chars`
- Graceful shutdown: daemon closes a `done` channel → context is cancelled → `b.Start(ctx)` returns
