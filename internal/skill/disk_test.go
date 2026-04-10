package skill

import (
	"testing"
)

func TestParseFrontmatter_WithFrontmatter(t *testing.T) {
	content := []byte(`---
name: sp-planning
description: Full planning process
category: methodology
---
# Planning
Some body text.`)
	name, desc, cat, body := ParseFrontmatter(content)
	if name != "sp-planning" {
		t.Errorf("expected name sp-planning, got %q", name)
	}
	if desc != "Full planning process" {
		t.Errorf("expected description, got %q", desc)
	}
	if cat != "methodology" {
		t.Errorf("expected category methodology, got %q", cat)
	}
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte(`# Just a header
No frontmatter here.`)
	name, desc, cat, body := ParseFrontmatter(content)
	if name != "" || desc != "" || cat != "" {
		t.Errorf("expected all empty, got name=%q desc=%q cat=%q", name, desc, cat)
	}
	if string(body) != string(content) {
		t.Errorf("expected body to be original content, got %q", string(body))
	}
}

func TestParseFrontmatter_CRLF(t *testing.T) {
	content := []byte("---\r\nname: crlf-test\r\n---\r\n# Header")
	name, _, _, _ := ParseFrontmatter(content)
	if name != "crlf-test" {
		t.Errorf("expected name crlf-test, got %q", name)
	}
}

func TestParseFrontmatter_NoTrailingNewline(t *testing.T) {
	content := []byte(`---
name: no-trailing
---
# No newline at end`)
	_, _, _, body := ParseFrontmatter(content)
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestParseFrontmatter_Unterminated(t *testing.T) {
	content := []byte(`---
name: broken
not closed`)
	name, desc, cat, body := ParseFrontmatter(content)
	if name != "" || desc != "" || cat != "" {
		t.Errorf("expected all empty on unterminated, got name=%q desc=%q cat=%q", name, desc, cat)
	}
	if string(body) != string(content) {
		t.Errorf("expected body to be original content on unterminated")
	}
}

func TestFetchContents_EmptyNames(t *testing.T) {
	content := FetchContents([]string{})
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}
