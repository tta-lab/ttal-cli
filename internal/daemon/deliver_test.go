package daemon

import "testing"

// TestFormatAgentMessage_HintIsItalic verifies that formatAgentMessage wraps the reply
// hint in literal <i>...</i> tags.
func TestFormatAgentMessage_HintIsItalic(t *testing.T) {
	got := formatAgentMessage("yuki", "hello")
	hint := "<i>--- Reply with: ttal send --to yuki \"your message\"</i>"
	if !contains(got, hint) {
		t.Errorf("expected italic hint %q in output:\n%s", hint, got)
	}
	// Should also contain the prefix
	if !contains(got, "[agent from:yuki]") {
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
