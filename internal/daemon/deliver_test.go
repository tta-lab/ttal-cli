package daemon

import (
	"strings"
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/sendfmt"
)

// TestFormatAgentMessage_HintIsItalic verifies that formatAgentMessage wraps the reply
// hint in literal <i>...</i> tags.
func TestFormatAgentMessage_HintIsItalic(t *testing.T) {
	got := formatAgentMessage("yuki", "hello")
	hint := "<i>--- Reply with:\ncat <<'EOF' | ttal send --to yuki\nyour message\nEOF</i>"
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
	want := `<i>--- Reply with:
cat <<'EOF' | ttal send --to astra
your message
EOF</i>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestFormatAgentMessage_NewSingleLineFormat pins the post-refactor layout —
// header and timestamp on one line, then body, then reply hint. Catches future
// reverts to the old two-line "[agent from:X]\n<body>" shape.
func TestFormatAgentMessage_NewSingleLineFormat(t *testing.T) {
	restore := sendfmt.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 5, 14, 32, 5, 0, time.UTC)
	})
	t.Cleanup(func() { restore() })

	got := formatAgentMessage("yuki", "hello")
	want := "[agent from:yuki] [14:32:05] hello\n\n" +
		`<i>--- Reply with:
cat <<'EOF' | ttal send --to yuki
your message
EOF</i>`
	if got != want {
		t.Errorf("formatAgentMessage mismatch\n  got:  %q\n  want: %q", got, want)
	}
}

func TestDeliverToAgent_LenosUsesTmux(t *testing.T) {
	cfg := &config.Config{DefaultRuntime: string(runtime.Lenos)}

	err := deliverToAgent(nil, cfg, nil, "no-such-lenos-test-agent", "hello")
	if err == nil {
		t.Fatal("expected tmux delivery error for missing test session, got nil")
	}
	if strings.Contains(err.Error(), "no frontend") {
		t.Fatalf("lenos delivery used frontend fallback instead of tmux: %v", err)
	}
}
