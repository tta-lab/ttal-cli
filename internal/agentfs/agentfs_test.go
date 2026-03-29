package agentfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	// Agent with frontmatter (flat .md file)
	yukiContent := []byte("---\nvoice: af_heart\nemoji: \U0001F431\n" +
		"description: Task orchestration\ncolor: green\n---\n# Yuki")
	os.WriteFile(filepath.Join(dir, "yuki.md"), yukiContent, 0o644) //nolint:errcheck

	// Agent without frontmatter (flat .md file)
	os.WriteFile(filepath.Join(dir, "kestrel.md"), []byte("# Kestrel\n\nA hawk agent."), 0o644)

	// Non-agent file (no .md extension)
	os.WriteFile(filepath.Join(dir, "README.txt"), []byte("readme"), 0o644)

	// Dot file (should be skipped)
	os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("# hidden"), 0o644)

	agents, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	// Find yuki
	var yAgent *AgentInfo
	for i := range agents {
		if agents[i].Name == "yuki" {
			yAgent = &agents[i]
		}
	}
	if yAgent == nil {
		t.Fatal("yuki not found")
		return
	}
	if yAgent.Voice != "af_heart" {
		t.Errorf("voice: got %q, want af_heart", yAgent.Voice)
	}
	if yAgent.Emoji != "\U0001F431" {
		t.Errorf("emoji: got %q, want cat emoji", yAgent.Emoji)
	}
	if yAgent.Description != "Task orchestration" {
		t.Errorf("description: got %q", yAgent.Description)
	}
	if yAgent.Color != "green" {
		t.Errorf("color: got %q, want green", yAgent.Color)
	}

	// Find kestrel (no frontmatter — fields should be empty)
	var kAgent *AgentInfo
	for i := range agents {
		if agents[i].Name == "kestrel" {
			kAgent = &agents[i]
		}
	}
	if kAgent == nil {
		t.Fatal("kestrel not found")
		return
	}
	if kAgent.Voice != "" || kAgent.Emoji != "" {
		t.Errorf("kestrel should have empty metadata, got voice=%q emoji=%q", kAgent.Voice, kAgent.Emoji)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "yuki.md"), []byte("---\nvoice: af_sky\ncolor: blue\n---\n# Yuki"), 0o644)

	ag, err := Get(dir, "yuki")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ag.Voice != "af_sky" {
		t.Errorf("voice: got %q, want af_sky", ag.Voice)
	}
	if ag.Color != "blue" {
		t.Errorf("color: got %q, want blue", ag.Color)
	}
}

func TestSetFieldColorRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Simple fixture — no nested YAML — to verify all fields survive a SetField rewrite
	fixture := []byte("---\nvoice: af_heart\nemoji: \U0001F431\n" +
		"description: Task orchestration\nrole: manager\ncolor: green\n---\n# Yuki\n\nSome content.")
	os.WriteFile(filepath.Join(dir, "yuki.md"), fixture, 0o644) //nolint:errcheck

	if err := SetField(dir, "yuki", "color", "cyan"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	ag, _ := Get(dir, "yuki")
	if ag.Color != "cyan" {
		t.Errorf("color after update: got %q, want cyan", ag.Color)
	}
	// Verify all other fields survived the rewrite
	if ag.Voice != "af_heart" {
		t.Errorf("voice should be preserved, got %q", ag.Voice)
	}
	if ag.Emoji != "\U0001F431" {
		t.Errorf("emoji should be preserved, got %q", ag.Emoji)
	}
	if ag.Description != "Task orchestration" {
		t.Errorf("description should be preserved, got %q", ag.Description)
	}
	if ag.Role != "manager" {
		t.Errorf("role should be preserved, got %q", ag.Role)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Get(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestCount(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		os.WriteFile(filepath.Join(dir, name+".md"), []byte("# "+name), 0o644)
	}
	// Non-agent
	os.WriteFile(filepath.Join(dir, "README.txt"), []byte("readme"), 0o644)

	n, err := Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestSetField(t *testing.T) {
	dir := t.TempDir()
	yukiContent := []byte("---\nvoice: af_heart\nemoji: \U0001F431\n---\n# Yuki\n\nSome content.")
	os.WriteFile(filepath.Join(dir, "yuki.md"), yukiContent, 0o644) //nolint:errcheck

	if err := SetField(dir, "yuki", "voice", "af_sky"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	ag, _ := Get(dir, "yuki")
	if ag.Voice != "af_sky" {
		t.Errorf("voice after update: got %q, want af_sky", ag.Voice)
	}
	if ag.Emoji != "\U0001F431" {
		t.Errorf("emoji should be preserved, got %q", ag.Emoji)
	}
}

func TestSetFieldNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "yuki.md"), []byte("# Yuki\n\nNo frontmatter here."), 0o644)

	if err := SetField(dir, "yuki", "voice", "af_heart"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	ag, _ := Get(dir, "yuki")
	if ag.Voice != "af_heart" {
		t.Errorf("voice: got %q, want af_heart", ag.Voice)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "yuki.md"))
	if !strings.Contains(string(data), "# Yuki") {
		t.Error("original content should be preserved")
	}
}
