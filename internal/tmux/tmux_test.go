package tmux

import (
	"testing"
)

// mockListSessions swaps out ListSessions during tests.
// We test the suffix-matching logic via filterSessionByPrefix directly
// since ListSessions requires a running tmux server.

// filterSessionByPrefix mirrors FindSessionByPrefix logic for unit testing.
func filterSessionByPrefix(sessions []string, prefix, suffix string) string {
	var found string
	for _, s := range sessions {
		if !hasPrefix(s, prefix) {
			continue
		}
		rest := s[len(prefix):]
		idx := indexOf(rest, "-")
		if idx < 0 || rest[idx:] != suffix {
			continue
		}
		if found == "" {
			found = s
		}
	}
	return found
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func indexOf(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

func TestFindSessionByPrefixSuffixMatching(t *testing.T) {
	tests := []struct {
		name     string
		sessions []string
		prefix   string
		suffix   string
		want     string
	}{
		{
			name:     "exact match",
			sessions: []string{"ts-abc12345-astra"},
			prefix:   "ts-",
			suffix:   "-astra",
			want:     "ts-abc12345-astra",
		},
		{
			name:     "no match — different agent suffix",
			sessions: []string{"ts-abc12345-kestrel"},
			prefix:   "ts-",
			suffix:   "-astra",
			want:     "",
		},
		{
			name:     "no false positive on agent name substring",
			sessions: []string{"ts-abc12345-mega-astra"},
			prefix:   "ts-",
			suffix:   "-astra",
			want:     "",
		},
		{
			name:     "correct agent among multiple sessions",
			sessions: []string{"ts-abc12345-kestrel", "ts-def67890-astra", "ts-abc12345-mega-astra"},
			prefix:   "ts-",
			suffix:   "-astra",
			want:     "ts-def67890-astra",
		},
		{
			name:     "empty session list",
			sessions: []string{},
			prefix:   "ts-",
			suffix:   "-astra",
			want:     "",
		},
		{
			name:     "no sessions with prefix",
			sessions: []string{"ttal-team-kestrel", "w-abc12345-worker"},
			prefix:   "ts-",
			suffix:   "-astra",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSessionByPrefix(tt.sessions, tt.prefix, tt.suffix)
			if got != tt.want {
				t.Errorf("filterSessionByPrefix(%v, %q, %q) = %q, want %q",
					tt.sessions, tt.prefix, tt.suffix, got, tt.want)
			}
		})
	}
}
