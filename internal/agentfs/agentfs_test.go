package agentfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testVoiceAfHeart = "af_heart"
	testEmojiCat     = "🐱"
)

// findAgent returns a pointer to the named agent in the slice, or nil.
func findAgent(agents []AgentInfo, name string) *AgentInfo {
	for i := range agents {
		if agents[i].Name == name {
			return &agents[i]
		}
	}
	return nil
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	// Agent with frontmatter (workspace subdir + AGENTS.md)
	yukiContent := []byte("---\nvoice: " + testVoiceAfHeart + "\nemoji: " + testEmojiCat + "\n" +
		"description: Task orchestration\ncolor: green\ndefault_runtime: codex\n---\n# Yuki")
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755)                            //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "yuki", "AGENTS.md"), yukiContent, 0o644) //nolint:errcheck

	// Agent without frontmatter (workspace subdir + AGENTS.md)
	os.MkdirAll(filepath.Join(dir, "kestrel"), 0o755)                                                     //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "kestrel", "AGENTS.md"), []byte("# Kestrel\n\nA hawk agent."), 0o644) //nolint:errcheck

	// Non-agent dir (no AGENTS.md — should be skipped)
	os.MkdirAll(filepath.Join(dir, "noidentity"), 0o755) //nolint:errcheck

	// Dot-prefixed dir (should be skipped)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)                                   //nolint:errcheck
	os.WriteFile(filepath.Join(dir, ".hidden", "AGENTS.md"), []byte("# hidden"), 0o644) //nolint:errcheck

	// Top-level .md file (should be skipped — only subdirs with AGENTS.md are agents)
	os.WriteFile(filepath.Join(dir, "README.txt"), []byte("readme"), 0o644)

	agents, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	yAgent := findAgent(agents, "yuki")
	if yAgent == nil {
		t.Fatal("yuki not found")
		return
	}
	if yAgent.Voice != testVoiceAfHeart {
		t.Errorf("voice: got %q, want af_heart", yAgent.Voice)
	}
	if yAgent.Emoji != testEmojiCat {
		t.Errorf("emoji: got %q, want cat emoji", yAgent.Emoji)
	}
	if yAgent.Description != "Task orchestration" {
		t.Errorf("description: got %q", yAgent.Description)
	}
	if yAgent.Color != "green" {
		t.Errorf("color: got %q, want green", yAgent.Color)
	}
	if yAgent.DefaultRuntime != "codex" {
		t.Errorf("default_runtime: got %q, want codex", yAgent.DefaultRuntime)
	}

	kAgent := findAgent(agents, "kestrel")
	if kAgent == nil {
		t.Fatal("kestrel not found")
		return
	}
	if kAgent.Voice != "" || kAgent.Emoji != "" || kAgent.Color != "" || kAgent.DefaultRuntime != "" {
		t.Errorf("kestrel should have empty metadata, got voice=%q emoji=%q color=%q default_runtime=%q",
			kAgent.Voice, kAgent.Emoji, kAgent.Color, kAgent.DefaultRuntime)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755) //nolint:errcheck
	f, _ := os.Create(filepath.Join(dir, "yuki", "AGENTS.md"))
	f.Write([]byte("---\nvoice: af_sky\ncolor: blue\n---\n# Yuki")) //nolint:errcheck
	f.Close()                                                       //nolint:errcheck

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
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755) //nolint:errcheck
	fixture := []byte("---\nvoice: " + testVoiceAfHeart + "\nemoji: " + testEmojiCat + "\n" +
		"description: Task orchestration\nrole: manager\ncolor: green\npronouns: she/her\nage: 25\n---\n# Yuki\n\nSome content.")
	os.WriteFile(filepath.Join(dir, "yuki", "AGENTS.md"), fixture, 0o644) //nolint:errcheck

	if err := SetField(dir, "yuki", "color", "cyan"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	ag, err := Get(dir, "yuki")
	if err != nil {
		t.Fatalf("Get after SetField: %v", err)
	}
	if ag.Color != "cyan" {
		t.Errorf("color after update: got %q, want cyan", ag.Color)
	}
	// Verify all other fields survived the rewrite
	if ag.Voice != testVoiceAfHeart {
		t.Errorf("voice should be preserved, got %q", ag.Voice)
	}
	if ag.Emoji != testEmojiCat {
		t.Errorf("emoji should be preserved, got %q", ag.Emoji)
	}
	if ag.Description != "Task orchestration" {
		t.Errorf("description should be preserved, got %q", ag.Description)
	}
	if ag.Role != "manager" {
		t.Errorf("role should be preserved, got %q", ag.Role)
	}
	if ag.Pronouns != "she/her" {
		t.Errorf("pronouns should be preserved, got %q", ag.Pronouns)
	}
	if ag.Age != 25 {
		t.Errorf("age should be preserved, got %d", ag.Age)
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
		os.MkdirAll(filepath.Join(dir, name), 0o755)                                  //nolint:errcheck
		os.WriteFile(filepath.Join(dir, name, "AGENTS.md"), []byte("# "+name), 0o644) //nolint:errcheck
	}
	// Non-agent dir (no AGENTS.md)
	os.MkdirAll(filepath.Join(dir, "noidentity"), 0o755) //nolint:errcheck

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
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755) //nolint:errcheck
	yukiContent := []byte("---\nvoice: " + testVoiceAfHeart + "\nemoji: " + testEmojiCat +
		"\n---\n# Yuki\n\nSome content.")
	os.WriteFile(filepath.Join(dir, "yuki", "AGENTS.md"), yukiContent, 0o644) //nolint:errcheck

	if err := SetField(dir, "yuki", "voice", "af_sky"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	ag, _ := Get(dir, "yuki")
	if ag.Voice != "af_sky" {
		t.Errorf("voice after update: got %q, want af_sky", ag.Voice)
	}
	if ag.Emoji != testEmojiCat {
		t.Errorf("emoji should be preserved, got %q", ag.Emoji)
	}
}

func TestSetFieldRejectsNestedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "coder"), 0o755) //nolint:errcheck
	nested := []byte("---\nname: coder\nrole: worker\nclaude-code:\n  model: sonnet\n---\n# Coder")
	os.WriteFile(filepath.Join(dir, "coder", "AGENTS.md"), nested, 0o644) //nolint:errcheck

	err := SetField(dir, "coder", "color", "green")
	if err == nil {
		t.Fatal("expected error for nested frontmatter, got nil")
	}
}

func TestSetFieldNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755) //nolint:errcheck
	f, _ := os.Create(filepath.Join(dir, "yuki", "AGENTS.md"))
	f.Write([]byte("# Yuki\n\nNo frontmatter here.")) //nolint:errcheck
	f.Close()                                         //nolint:errcheck

	if err := SetField(dir, "yuki", "voice", testVoiceAfHeart); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	ag, _ := Get(dir, "yuki")
	if ag.Voice != testVoiceAfHeart {
		t.Errorf("voice: got %q, want af_heart", ag.Voice)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "yuki", "AGENTS.md"))
	if !strings.Contains(string(data), "# Yuki") {
		t.Error("original content should be preserved")
	}
}

func TestDiscoverAgents(t *testing.T) {
	dir := t.TempDir()
	// 3 valid agent dirs
	for _, name := range []string{"yuki", "kestrel", "athena"} {
		os.MkdirAll(filepath.Join(dir, name), 0o755)                                  //nolint:errcheck
		os.WriteFile(filepath.Join(dir, name, "AGENTS.md"), []byte("# "+name), 0o644) //nolint:errcheck
	}
	// Dir without AGENTS.md — should be skipped
	os.MkdirAll(filepath.Join(dir, "noidentity"), 0o755) //nolint:errcheck
	// Dot dir — should be skipped
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)                                   //nolint:errcheck
	os.WriteFile(filepath.Join(dir, ".hidden", "AGENTS.md"), []byte("# hidden"), 0o644) //nolint:errcheck

	names, err := DiscoverAgents(dir)
	if err != nil {
		t.Fatalf("DiscoverAgents: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 agents, got %d: %v", len(names), names)
	}
}

func TestHasAgent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755)                                 //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "yuki", "AGENTS.md"), []byte("# Yuki"), 0o644) //nolint:errcheck
	os.MkdirAll(filepath.Join(dir, "noidentity"), 0o755)                           //nolint:errcheck

	if !HasAgent(dir, "yuki") {
		t.Error("HasAgent(yuki) should be true")
	}
	if HasAgent(dir, "nonexistent") {
		t.Error("HasAgent(nonexistent) should be false")
	}
	if HasAgent(dir, "noidentity") {
		t.Error("HasAgent(noidentity) should be false (dir exists but no AGENTS.md)")
	}
}

func TestGetFromPath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "yuki"), 0o755) //nolint:errcheck
	f, _ := os.Create(filepath.Join(dir, "yuki", "AGENTS.md"))
	f.Write([]byte("---\nvoice: af_heart\nemoji: 🐱\n---\n# Yuki")) //nolint:errcheck
	f.Close()                                                      //nolint:errcheck

	agentDir := filepath.Join(dir, "yuki")
	ag, err := GetFromPath(agentDir)
	if err != nil {
		t.Fatalf("GetFromPath: %v", err)
	}
	if ag.Name != "yuki" {
		t.Errorf("name: got %q, want yuki", ag.Name)
	}
	if ag.Voice != "af_heart" {
		t.Errorf("voice: got %q, want af_heart", ag.Voice)
	}
}

// TestAgentFS_RequiresSubdirLayout documents the expected layout contract:
// agentfs.GetFromPaths requires {name}/AGENTS.md and never accepts flat .md files.
// This test locks in the intentional break — flat files were never supported
// by GetFromPaths, and the bug was that workers were stored flat, not that
// GetFromPaths ever accepted them.
func TestAgentFS_RequiresSubdirLayout(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a flat agents/coder.md (the old, broken layout).
	if err := os.WriteFile(filepath.Join(tmpDir, "coder.md"), []byte("---\nname: coder\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := GetFromPaths([]string{tmpDir}, "coder")
	if err == nil {
		t.Error("expected error for flat layout, got nil — flat .md files are not supported by GetFromPaths")
	}
}
