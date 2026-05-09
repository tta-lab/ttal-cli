package sendfmt

import (
	"fmt"
	"strings"
	"time"
)

// nowFn is the package clock; tests stub this for deterministic output.
var nowFn = time.Now

// Envelope describes a delivery message before rendering.
type Envelope struct {
	Channel    string    // "agent" | "telegram" | "matrix" | "" (no header)
	SenderName string    // displayed after "from:"; required when Channel != ""
	Body       string    // user-supplied content
	ReplyAlias string    // shown in reply hint; "" -> no reply hint
	Now        time.Time // timestamp source; zero value -> nowFn()
}

// Format renders the envelope as:
//
//	[<channel> from:<sender>] [<hh:mm:ss>] <body>
//
//	<i>--- Reply with:
//	cat <<'EOF' | ttal send --to <alias>
//	your message
//	EOF</i>
//
// Header is omitted when Channel == ""; reply hint is omitted when ReplyAlias == "".
// Body is preserved verbatim (no escaping, no rewriting).
func Format(env Envelope) string {
	now := env.Now
	if now.IsZero() {
		now = nowFn()
	}
	var head []string
	if env.Channel != "" && env.SenderName != "" {
		head = append(head, fmt.Sprintf("[%s from:%s]", env.Channel, env.SenderName))
	}
	head = append(head, fmt.Sprintf("[%s]", now.Format("15:04:05")))
	head = append(head, env.Body)
	out := strings.Join(head, " ")
	if env.ReplyAlias != "" {
		out += "\n\n" + ReplyHint(env.ReplyAlias)
	}
	return out
}

// ReplyHint returns the literal italic reply-hint footer used across deliveries.
// Exposed for callers (e.g. cmd/send.go) that compose their own message but
// still want the canonical hint string.
func ReplyHint(alias string) string {
	return fmt.Sprintf(`<i>--- Reply with:
cat <<'EOF' | ttal send --to %s
your message
EOF</i>`, alias)
}

// SetNowForTest replaces the package clock; returns the previous value so tests
// can restore it. Test-only helper — do not call from production code.
func SetNowForTest(fn func() time.Time) func() time.Time {
	prev := nowFn
	nowFn = fn
	return prev
}
