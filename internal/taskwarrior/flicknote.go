package taskwarrior

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

var inlineRefPattern = regexp.MustCompile(`(?:Plan|Design):\s*([~\/][\w\/\-\.]+\.md)`)

var referenceRefPattern = regexp.MustCompile(`(?:Research|Doc|Reference|File):\s*([~\/][\w\/\-\.]+\.md)`)

var rawPathPattern = regexp.MustCompile(`^([~\/][\w\/\-\.]+\.md)$`)

type FlicknoteNote struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Project string `json:"project"`
	Summary string `json:"summary"`
	Content string `json:"content"`
}

func ReadFlicknoteJSON(id string) *FlicknoteNote {
	ctx, cancel := context.WithTimeout(context.Background(), flicknoteTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "flicknote", "detail", "--json", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("flicknote detail %s failed: %v", id, err)
		return nil
	}

	var note FlicknoteNote
	if err := json.Unmarshal(out, &note); err != nil {
		log.Printf("flicknote detail %s: failed to parse JSON: %v", id, err)
		return nil
	}
	return &note
}

func ShouldInlineNote(note *FlicknoteNote, inlineProjects []string) bool {
	name := strings.ToLower(note.Project)
	for _, keyword := range inlineProjects {
		if strings.Contains(name, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func LoadInlineProjects() []string {
	cfg, err := config.Load()
	if err == nil && len(cfg.Flicknote.InlineProjects) > 0 {
		return cfg.Flicknote.InlineProjects
	}
	return config.DefaultInlineProjects
}

func formatFlicknoteContent(note *FlicknoteNote) string {
	var b strings.Builder
	b.WriteString("Title: " + note.Title + "\n")
	if note.Summary != "" {
		b.WriteString("Summary: " + note.Summary + "\n")
	}
	if note.Content != "" {
		b.WriteString("\n" + note.Content)
	}
	return b.String()
}

type docRef struct {
	label   string
	refType string
	id      string
}

func (t *Task) FormatPrompt() string {
	lines := make([]string, 0, 1+len(t.Annotations))
	lines = append(lines, t.Description)

	inlineProjects := LoadInlineProjects()

	refDescs := make(map[string]bool)
	flicknoteCache := make(map[string]string)
	var refs []docRef

	for _, ann := range t.Annotations {
		desc := ann.Description

		if matches := inlineRefPattern.FindAllStringSubmatch(desc, -1); len(matches) > 0 {
			for _, m := range matches {
				refDescs[desc] = true
				refs = append(refs, docRef{label: desc, refType: "file", id: m[1]})
			}
			continue
		}

		if referenceRefPattern.MatchString(desc) {
			continue
		}

		if m := HexIDPattern.FindStringSubmatch(desc); len(m) > 0 {
			hexID := m[1]
			refDescs[desc] = true

			note := ReadFlicknoteJSON(hexID)
			if note != nil && ShouldInlineNote(note, inlineProjects) {
				flicknoteCache[desc] = formatFlicknoteContent(note)
				refs = append(refs, docRef{label: "FlickNote: " + hexID, refType: "flicknote_cached", id: desc})
			}
			continue
		}

		if matches := rawPathPattern.FindStringSubmatch(desc); len(matches) > 0 {
			refDescs[desc] = true
			refs = append(refs, docRef{label: desc, refType: "file", id: matches[1]})
			continue
		}
	}

	for _, ann := range t.Annotations {
		if refDescs[ann.Description] {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, ann.Description)
	}

	result := strings.Join(lines, "\n") + "\n"

	if len(refs) > 0 {
		result += "\nReferenced Documentation:\n"
		sep := strings.Repeat("═", 80)
		subSep := strings.Repeat("─", 80)
		for _, ref := range refs {
			result += sep + "\n"
			result += ref.label + "\n"
			result += subSep + "\n"
			switch ref.refType {
			case "file":
				result += readFileRef(ref.id) + "\n"
			case "flicknote_cached":
				result += flicknoteCache[ref.id] + "\n"
			}
			result += sep + "\n"
		}
	}

	if IsFork() {
		result = appendSubtasksSection(result, t.UUID)
	}
	if IsFork() && t.ParentID != "" {
		result = prependParentContext(result, t)
	}

	return result
}

// taskStatusLabel returns a human-readable status label for prompt output.
func taskStatusLabel(t *Task) string {
	if t.Status == "completed" {
		return "✓ done"
	}
	if t.IsActive() {
		return "● active"
	}
	return "pending"
}

// taskStatusGlyph returns a short status glyph ("✓" / "●" / "").
func taskStatusGlyph(t *Task) string {
	if t.Status == "completed" {
		return "✓"
	}
	if t.IsActive() {
		return "●"
	}
	return ""
}

// isPipelineAnnotation returns true for annotation prefixes that are pipeline noise.
func isPipelineAnnotation(desc string) bool {
	return strings.HasPrefix(desc, "pipeline:") ||
		strings.HasPrefix(desc, "advanced:") ||
		strings.HasPrefix(desc, "lgtm:") ||
		strings.HasPrefix(desc, "stage:")
}

// appendSubtasksSection appends a formatted subtask tree to result.
func appendSubtasksSection(result, parentUUID string) string {
	children, err := GetChildren(parentUUID)
	if err != nil {
		return result + fmt.Sprintf("\nSubtasks: [error loading: %v]\n", err)
	}
	if len(children) == 0 {
		return result
	}
	result += "\nSubtasks:\n" + strings.Repeat("─", 80) + "\n"
	for i, child := range children {
		prefix := "├─"
		if i == len(children)-1 {
			prefix = "└─"
		}
		result += fmt.Sprintf("%s [%s] %s (%s)\n", prefix, child.HexID(), child.Description, taskStatusLabel(&child))
		for _, ann := range child.Annotations {
			if !isPipelineAnnotation(ann.Description) {
				result += fmt.Sprintf("   %s\n", ann.Description)
			}
		}
	}
	return result + strings.Repeat("─", 80) + "\n"
}

// prependParentContext prepends parent description and appends siblings when t is a subtask.
func prependParentContext(result string, t *Task) string {
	parent, err := ExportTask(t.ParentID)
	if err != nil {
		result = fmt.Sprintf("Parent: [error loading %s: %v]\n\n", t.ParentID[:8], err) + result
	} else {
		result = fmt.Sprintf("Parent: [%s] %s\n\n", parent.HexID(), parent.Description) + result
	}

	siblings, err := GetChildren(t.ParentID)
	if err != nil {
		return result + fmt.Sprintf("\nSibling tasks: [error loading: %v]\n", err)
	}
	if len(siblings) <= 1 {
		return result
	}
	result += "\nSibling tasks:\n"
	for _, sib := range siblings {
		if sib.UUID == t.UUID {
			result += fmt.Sprintf("  → [%s] %s (this task)\n", sib.HexID(), sib.Description)
		} else {
			glyph := taskStatusGlyph(&sib)
			if glyph != "" {
				result += fmt.Sprintf("    [%s] %s (%s)\n", sib.HexID(), sib.Description, glyph)
			} else {
				result += fmt.Sprintf("    [%s] %s (pending)\n", sib.HexID(), sib.Description)
			}
		}
	}
	return result
}

func readFileRef(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Sprintf("[Error expanding home directory: %v]", err)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("[File not found: %s]", path)
	}
	return string(data)
}
