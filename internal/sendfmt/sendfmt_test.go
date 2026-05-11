package sendfmt

import (
	"regexp"
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestFormat_AllShapes(t *testing.T) {
	fixed := time.Date(2026, 5, 5, 14, 32, 5, 0, time.FixedZone("UTC+8", 8*60*60))
	replyHintYuki := `<i>--- Reply with:
cat <<'EOF' | ttal send --to yuki
your message
EOF</i>`
	replyHintNeil := `<i>--- Reply with:
cat <<'EOF' | ttal send --to neil
your message
EOF</i>`
	cases := []struct {
		name string
		env  Envelope
		want string
	}{
		{
			name: "bare (no header, no reply hint) — used for ttal send from bare shell",
			env:  Envelope{Body: "hello", Now: fixed},
			want: "[14:32:05] hello",
		},
		{
			name: "agent-to-agent — full wrap with reply hint",
			env: Envelope{
				Channel: "agent", SenderName: "yuki",
				Body: "hello", ReplyAlias: "yuki", Now: fixed,
			},
			want: `<- yuki [14:32:05] hello` + "\n\n" + replyHintYuki,
		},
		{
			name: "telegram inbound — header + reply hint to admin",
			env: Envelope{
				Channel: "telegram", SenderName: "Neil",
				Body: "hello", ReplyAlias: "neil", Now: fixed,
			},
			want: `<- telegram:Neil [14:32:05] hello` + "\n\n" + replyHintNeil,
		},
		{
			name: "matrix inbound — header + reply hint to admin",
			env: Envelope{
				Channel: "matrix", SenderName: "Neil",
				Body: "hello", ReplyAlias: "neil", Now: fixed,
			},
			want: `<- matrix:Neil [14:32:05] hello` + "\n\n" + replyHintNeil,
		},
		{
			name: "header without reply hint — defensive shape, no current caller",
			env: Envelope{
				Channel: "agent", SenderName: "yuki",
				Body: "hello", Now: fixed,
			},
			want: `<- yuki [14:32:05] hello`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Format(tc.env)
			if got != tc.want {
				t.Errorf("Format mismatch\n  got:  %q\n  want: %q", got, tc.want)
			}
		})
	}
}

func TestFormat_UsesLocalTimeNotUTC(t *testing.T) {
	loc := time.FixedZone("UTC-16", -16*60*60)
	t1 := time.Date(2026, 5, 5, 6, 32, 5, 0, loc)
	got := Format(Envelope{Body: "hi", Now: t1})
	if got != "[06:32:05] hi" {
		t.Errorf("expected local-time prefix [06:32:05], got %q", got)
	}
	if got == "[22:32:05] hi" {
		t.Fatalf("formatter normalized to UTC — local-time contract violated")
	}
}

func TestFormat_NowFnFallback(t *testing.T) {
	orig := nowFn
	t.Cleanup(func() { nowFn = orig })
	nowFn = func() time.Time {
		return time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	}
	got := Format(Envelope{Body: "x"})
	if got != "[09:00:00] x" {
		t.Errorf("nowFn fallback failed: %q", got)
	}
}

func TestFormat_HeaderShape(t *testing.T) {
	got := Format(Envelope{
		Channel: "agent", SenderName: "yuki", Body: "any body content",
		ReplyAlias: "yuki", Now: time.Now(),
	})
	re := regexp.MustCompile(`^<- yuki \[\d{2}:\d{2}:\d{2}\] any body content`)
	if !re.MatchString(got) {
		t.Errorf("header shape mismatch: %q", got)
	}
}

func TestReplyHint(t *testing.T) {
	got := ReplyHint("neil")
	want := `<i>--- Reply with:
cat <<'EOF' | ttal send --to neil
your message
EOF</i>`
	if got != want {
		t.Errorf("ReplyHint = %q, want %q", got, want)
	}
}

func TestReplyHintForRuntime_LenosUsesNarrate(t *testing.T) {
	got := ReplyHintForRuntime("neil", runtime.Lenos)
	want := `<i>--- Reply with:
narrate --to neil <<'EOF'
your message
EOF</i>`
	if got != want {
		t.Errorf("ReplyHintForRuntime = %q, want %q", got, want)
	}
}
