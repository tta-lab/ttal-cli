// Package telegram provides Telegram Bot API helpers for sending messages and managing interactions.
//
// It wraps the go-telegram/bot library to send text messages, voice (OGG) messages,
// and emoji reactions. It also provides rendering helpers for interactive question
// pages with inline keyboards, used by the daemon to surface AskUserQuestion prompts
// from agent sessions to the human operator over Telegram.
//
// Plane: shared
package telegram
