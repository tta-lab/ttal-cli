package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/skill"
)

func skillContent(name, desc, category, body string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\ncategory: " + category + "\n---\n" + body
}

func writeSkillFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeSkillFile failed: %v", err)
	}
}

func TestSkillList_TableOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	planning := skillContent("sp-planning", "Full planning process", "methodology", "# Planning\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)
	writeSkillFile(t, dir, "sp-planning", planning)

	err := runSkillList(false, false)
	if err != nil {
		t.Fatalf("runSkillList failed: %v", err)
	}
}

func TestSkillList_TableWithCategories(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)

	err := runSkillList(true, false)
	if err != nil {
		t.Fatalf("runSkillList with listAll=true failed: %v", err)
	}
}

func TestSkillList_JSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	planning := skillContent("sp-planning", "Full planning", "methodology", "# Planning\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)
	writeSkillFile(t, dir, "sp-planning", planning)

	err := runSkillList(false, true)
	if err != nil {
		t.Fatalf("runSkillList JSON failed: %v", err)
	}
}

func TestSkillGet_ContentOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	content := skillContent("test-skill", "A test skill", "command",
		"# Test Skill Body\n\nThis is the actual content.")
	writeSkillFile(t, dir, "test-skill", content)

	err := runSkillGet("test-skill", false)
	if err != nil {
		t.Fatalf("runSkillGet failed: %v", err)
	}
}

func TestSkillGet_JSONWithContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	content := skillContent("test-skill", "A test skill", "command",
		"# Test Skill Body\n\nThis is the actual content.")
	writeSkillFile(t, dir, "test-skill", content)

	err := runSkillGet("test-skill", true)
	if err != nil {
		t.Fatalf("runSkillGet JSON failed: %v", err)
	}
}

func TestSkillGet_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	err := runSkillGet("nonexistent-skill", false)
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestSkillFind_ByName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	planning := skillContent("sp-planning", "Full planning", "methodology", "# Planning\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)
	writeSkillFile(t, dir, "sp-planning", planning)

	err := runSkillFind([]string{"breathe"})
	if err != nil {
		t.Fatalf("runSkillFind failed: %v", err)
	}
}

func TestSkillFind_ByDescription(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)

	err := runSkillFind([]string{"context"})
	if err != nil {
		t.Fatalf("runSkillFind by description failed: %v", err)
	}
}

func TestSkillFind_NoMatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)

	err := runSkillFind([]string{"nonexistent"})
	if err != nil {
		t.Fatalf("runSkillFind no match failed: %v", err)
	}
}

func TestFindByNameDesc_Direct(t *testing.T) {
	candidates := []skill.DiskSkill{
		{Name: "sp-planning", Description: "Full planning process", Category: "methodology"},
		{Name: "breathe", Description: "Refresh context window", Category: "command"},
		{Name: "sp-debugging", Description: "Diagnose bugs systematically", Category: "methodology"},
	}

	results := findByNameDesc(candidates, []string{"planning"})
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'planning', got %d", len(results))
	}
	if _, ok := results["sp-planning"]; !ok {
		t.Error("expected sp-planning in results")
	}

	results = findByNameDesc(candidates, []string{"bugs"})
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'bugs', got %d", len(results))
	}

	results = findByNameDesc(candidates, []string{"nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'nonexistent', got %d", len(results))
	}
}

func TestSkillList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	err := runSkillList(false, false)
	if err != nil {
		t.Fatalf("runSkillList on empty dir failed: %v", err)
	}
}

func TestSkillGet_FileWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	writeSkillFile(t, dir, "plain-skill", "Plain skill body content.")

	err := runSkillGet("plain-skill", false)
	if err != nil {
		t.Fatalf("runSkillGet without frontmatter failed: %v", err)
	}
}

func TestSkillList_JSON_Structure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	writeSkillFile(t, dir, "breathe", breathe)

	old := os.Stdout
	rPipe, wPipe, _ := os.Pipe()
	os.Stdout = wPipe

	err := runSkillList(false, true)

	wPipe.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runSkillList JSON failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := rPipe.Read(buf)
	output := string(buf[:n])

	var items []skillJSON
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 skill, got %d", len(items))
	}
	if items[0].Name != "breathe" {
		t.Errorf("expected name 'breathe', got %q", items[0].Name)
	}
	if items[0].Category != "command" {
		t.Errorf("expected category 'command', got %q", items[0].Category)
	}
}

func TestSkillGet_JSON_Structure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	body := skillContent("test-skill", "A test skill", "command",
		"# Test Skill Body\n\nThis is the actual content.")
	writeSkillFile(t, dir, "test-skill", body)

	old := os.Stdout
	rPipe, wPipe, _ := os.Pipe()
	os.Stdout = wPipe

	err := runSkillGet("test-skill", true)

	wPipe.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runSkillGet JSON failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := rPipe.Read(buf)
	output := string(buf[:n])

	var item skillJSONWithContent
	if err := json.Unmarshal([]byte(output), &item); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
	}

	if item.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", item.Name)
	}
	if item.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestBuildSkillTable_DimColumns(t *testing.T) {
	headers := []string{"Name", "Category", "Description"}
	rows := [][]string{
		{"breathe", "command", "Refresh context"},
	}
	table := buildSkillTable(headers, rows, 1)
	if table == nil {
		t.Error("expected non-nil table")
	}
}

func TestBuildSkillTable_NoDimColumns(t *testing.T) {
	headers := []string{"Name", "Description"}
	rows := [][]string{
		{"breathe", "Refresh context"},
	}
	table := buildSkillTable(headers, rows)
	if table == nil {
		t.Error("expected non-nil table")
	}
}

func TestSkillFind_MultipleKeywords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TTAL_SKILLS_DIR", dir)

	planning := skillContent("sp-planning", "Full planning process", "methodology", "# Planning\nBody here.")
	debugging := skillContent("sp-debugging", "Debug systematically", "methodology", "# Debug\nBody here.")
	breathe := skillContent("breathe", "Refresh context", "command", "# Breathe\nBody here.")
	writeSkillFile(t, dir, "sp-planning", planning)
	writeSkillFile(t, dir, "sp-debugging", debugging)
	writeSkillFile(t, dir, "breathe", breathe)

	err := runSkillFind([]string{"methodology"})
	if err != nil {
		t.Fatalf("runSkillFind multiple keywords failed: %v", err)
	}
}

func TestFindByNameDesc_CaseInsensitive(t *testing.T) {
	candidates := []skill.DiskSkill{
		{Name: "Sp-Planning", Description: "Full Planning Process", Category: "methodology"},
	}

	results := findByNameDesc(candidates, []string{"PLANNING"})
	if len(results) != 1 {
		t.Errorf("expected 1 result for case-insensitive match, got %d", len(results))
	}
}

func TestFindByNameDesc_MultipleMatchesSameSkill(t *testing.T) {
	candidates := []skill.DiskSkill{
		{Name: "sp-planning", Description: "Full planning process", Category: "methodology"},
	}

	results := findByNameDesc(candidates, []string{"planning", "process", "full"})
	if len(results) != 1 {
		t.Errorf("expected 1 result (deduplicated), got %d", len(results))
	}
}
