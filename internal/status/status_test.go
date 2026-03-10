package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testTeam = "guion"

// writeJunkFiles creates non-agent files that readAllFrom should skip.
func writeJunkFiles(t *testing.T, dir string) {
	t.Helper()
	// bare .json (no name) — should skip without panic
	os.WriteFile(filepath.Join(dir, ".json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
}

func writeStatusFile(t *testing.T, dir, filename string, s AgentStatus) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsStale(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt time.Time
		threshold time.Duration
		want      bool
	}{
		{
			name:      "fresh status",
			updatedAt: time.Now().Add(-1 * time.Minute),
			threshold: 5 * time.Minute,
			want:      false,
		},
		{
			name:      "stale status",
			updatedAt: time.Now().Add(-10 * time.Minute),
			threshold: 5 * time.Minute,
			want:      true,
		},
		{
			name:      "recent update with large threshold",
			updatedAt: time.Now(),
			threshold: 1 * time.Hour,
			want:      false,
		},
		{
			name:      "zero time is stale",
			updatedAt: time.Time{},
			threshold: 5 * time.Minute,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AgentStatus{UpdatedAt: tt.updatedAt}
			if got := s.IsStale(tt.threshold); got != tt.want {
				t.Errorf("IsStale(%v) = %v, want %v", tt.threshold, got, tt.want)
			}
		})
	}
}

func TestReadAgentFrom(t *testing.T) {
	dir := t.TempDir()
	team := testTeam

	t.Run("file not found returns nil", func(t *testing.T) {
		s, err := readAgentFrom(dir, team, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != nil {
			t.Fatal("expected nil for missing file")
		}
	})

	t.Run("valid JSON", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		want := AgentStatus{
			Agent:               "athena",
			ContextUsedPct:      67.5,
			ContextRemainingPct: 32.5,
			ModelID:             "claude-opus-4-6",
			ModelName:           "Claude Opus 4.6",
			SessionID:           "abc123",
			CCVersion:           "1.0.0",
			UpdatedAt:           now,
		}
		writeStatusFile(t, dir, "guion-athena", want)

		got, err := readAgentFrom(dir, team, "athena")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil status")
		}
		if got.Agent != want.Agent {
			t.Errorf("Agent = %q, want %q", got.Agent, want.Agent)
		}
		if got.ContextUsedPct != want.ContextUsedPct {
			t.Errorf("ContextUsedPct = %v, want %v", got.ContextUsedPct, want.ContextUsedPct)
		}
		if got.ModelID != want.ModelID {
			t.Errorf("ModelID = %q, want %q", got.ModelID, want.ModelID)
		}
		if got.SessionID != want.SessionID {
			t.Errorf("SessionID = %q, want %q", got.SessionID, want.SessionID)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "guion-broken.json"), []byte("{invalid"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := readAgentFrom(dir, team, "broken")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestReadAllFrom_Empty(t *testing.T) {
	dir := t.TempDir()
	statuses, err := readAllFrom(dir, testTeam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestReadAllFrom_NonexistentDir(t *testing.T) {
	statuses, err := readAllFrom("/tmp/nonexistent-ttal-test-dir", testTeam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statuses != nil {
		t.Errorf("expected nil for nonexistent dir, got %v", statuses)
	}
}

func TestReadAllFrom_MultipleAgents(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	writeStatusFile(t, dir, "guion-athena", AgentStatus{Agent: "athena", ContextUsedPct: 50, UpdatedAt: now})
	writeStatusFile(t, dir, "guion-kestrel", AgentStatus{Agent: "kestrel", ContextUsedPct: 75, UpdatedAt: now})

	statuses, err := readAllFrom(dir, testTeam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	agents := map[string]float64{}
	for _, s := range statuses {
		agents[s.Agent] = s.ContextUsedPct
	}
	if agents["athena"] != 50 {
		t.Errorf("athena ContextUsedPct = %v, want 50", agents["athena"])
	}
	if agents["kestrel"] != 75 {
		t.Errorf("kestrel ContextUsedPct = %v, want 75", agents["kestrel"])
	}
}

func TestReadAllFrom_FiltersByTeam(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	writeStatusFile(t, dir, "guion-athena", AgentStatus{Agent: "athena", ContextUsedPct: 50, UpdatedAt: now})
	writeStatusFile(t, dir, "sven-athena", AgentStatus{Agent: "athena", ContextUsedPct: 80, UpdatedAt: now})
	writeStatusFile(t, dir, "guion-kestrel", AgentStatus{Agent: "kestrel", ContextUsedPct: 75, UpdatedAt: now})

	guionStatuses, err := readAllFrom(dir, testTeam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(guionStatuses) != 2 {
		t.Fatalf("expected 2 guion statuses, got %d", len(guionStatuses))
	}

	svenStatuses, err := readAllFrom(dir, "sven")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svenStatuses) != 1 {
		t.Fatalf("expected 1 sven status, got %d", len(svenStatuses))
	}
	if svenStatuses[0].ContextUsedPct != 80 {
		t.Errorf("sven athena ContextUsedPct = %v, want 80", svenStatuses[0].ContextUsedPct)
	}
}

func TestReadAllFrom_SkipsJunk(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	writeStatusFile(t, dir, "guion-good", AgentStatus{Agent: "good", UpdatedAt: now})
	writeJunkFiles(t, dir)

	statuses, err := readAllFrom(dir, testTeam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Agent != "good" {
		t.Errorf("expected agent 'good', got %q", statuses[0].Agent)
	}
}

func TestWriteAgentTo_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	want := AgentStatus{
		Agent:               "kestrel",
		ContextUsedPct:      42,
		ContextRemainingPct: 58,
		ModelID:             "claude-opus-4-6",
		ModelName:           "Claude Opus 4.6",
		SessionID:           "sess1",
		CCVersion:           "1.0.20",
		UpdatedAt:           now,
	}

	if err := writeAgentTo(dir, testTeam, want); err != nil {
		t.Fatalf("writeAgentTo: %v", err)
	}

	got, err := readAgentFrom(dir, testTeam, "kestrel")
	if err != nil {
		t.Fatalf("readAgentFrom: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil status")
	}
	if got.Agent != want.Agent {
		t.Errorf("Agent = %q, want %q", got.Agent, want.Agent)
	}
	if got.ContextUsedPct != want.ContextUsedPct {
		t.Errorf("ContextUsedPct = %v, want %v", got.ContextUsedPct, want.ContextUsedPct)
	}
	if got.ModelID != want.ModelID {
		t.Errorf("ModelID = %q, want %q", got.ModelID, want.ModelID)
	}
	if got.SessionID != want.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, want.SessionID)
	}
	if got.CCVersion != want.CCVersion {
		t.Errorf("CCVersion = %q, want %q", got.CCVersion, want.CCVersion)
	}

	// Verify tmp file is cleaned up
	tmpPath := filepath.Join(dir, ".guion-kestrel.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("expected tmp file to be cleaned up after rename")
	}

	// Verify file is at team-prefixed path
	target := filepath.Join(dir, "guion-kestrel.json")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected file at %s, got error: %v", target, err)
	}
}

func TestWriteAgentTo_RejectsInvalidAgent(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"", "../evil", "a/b", ".hidden", "..\\evil"} {
		s := AgentStatus{Agent: bad}
		if err := writeAgentTo(dir, testTeam, s); err == nil {
			t.Errorf("expected error for agent name %q", bad)
		}
	}
}

func TestWriteAgentTo_RejectsInvalidTeam(t *testing.T) {
	dir := t.TempDir()
	s := AgentStatus{Agent: "bot"}
	for _, bad := range []string{"", "../evil", "a/b", ".hidden"} {
		if err := writeAgentTo(dir, bad, s); err == nil {
			t.Errorf("expected error for team name %q", bad)
		}
	}
}

func TestWriteAgentTo_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "status")
	s := AgentStatus{Agent: "bot", UpdatedAt: time.Now().UTC()}

	if err := writeAgentTo(dir, testTeam, s); err != nil {
		t.Fatalf("writeAgentTo: %v", err)
	}

	got, err := readAgentFrom(dir, testTeam, "bot")
	if err != nil {
		t.Fatalf("readAgentFrom: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil status")
	}
}

func TestStatusFileName(t *testing.T) {
	tests := []struct {
		team, agent, want string
	}{
		{"guion", "athena", "guion-athena"},
		{"sven", "kestrel", "sven-kestrel"},
		{"default", "bot", "default-bot"},
	}
	for _, tt := range tests {
		got := statusFileName(tt.team, tt.agent)
		if got != tt.want {
			t.Errorf("statusFileName(%q, %q) = %q, want %q", tt.team, tt.agent, got, tt.want)
		}
	}
}
