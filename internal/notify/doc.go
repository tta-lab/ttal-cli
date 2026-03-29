// Package notify provides fire-and-forget Telegram notifications for the active team.
//
// It wraps the telegram package with two entry points: Send (loads config automatically)
// and SendWithConfig (accepts pre-resolved token and chat ID for daemon use). Both
// deliver messages to the team's configured notification bot and chat ID.
//
// Migration note: prefer daemon.Notify() from CLI/worker contexts, or fe.SendNotification()
// from daemon-internal code. This package hardcodes Telegram as the transport.
//
// Plane: shared
package notify
