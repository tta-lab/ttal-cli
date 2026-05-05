// Package sendfmt renders the canonical delivery format for ttal send
// messages: optional [<channel> from:<sender>] header, mandatory
// [<hh:mm:ss>] local-time stamp, body, and optional reply hint.
//
// All three duplicate formatters (daemon.formatAgentMessage,
// frontend/telegram.formatInboundMessage, frontend/matrix inline Sprintf)
// route through Format. CLI- and frontend-side callers compose Envelope
// values; the daemon decides whether to wrap (UserInitiated flag).
//
// Plane: shared
package sendfmt
