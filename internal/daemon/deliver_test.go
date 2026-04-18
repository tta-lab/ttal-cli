package daemon

import (
	"strings"
	"testing"
)

// TestFormatAgentMessage_HintIsItalic verifies that formatAgentMessage wraps the reply
// hint in literal <i>...</i> tags.
func TestFormatAgentMessage_HintIsItalic(t *testing.T) {
	got := formatAgentMessage("yuki", "hello")
	hint := "<i>--- Reply with: ttal send --to yuki \"your message\"</i>"
	if !strings.Contains(got, hint) {
		t.Errorf("expected italic hint %q in output:\n%s", hint, got)
	}
	if !strings.Contains(got, "[agent from:yuki]") {
		t.Errorf("expected [agent from:yuki] prefix in:\n%s", got)
	}
}

// TestReplyHint_HintIsItalic verifies ReplyHint itself wraps in italic tags.
func TestReplyHint_HintIsItalic(t *testing.T) {
	got := ReplyHint("astra")
	want := `<i>--- Reply with: ttal send --to astra "your message"</i>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
