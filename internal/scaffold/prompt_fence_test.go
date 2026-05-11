package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPromptSourcesDoNotContainCopyableFences(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bashFenceOffenders, promptFenceOffenders, err := findCopyableFenceOffenders(root)
	if err != nil {
		t.Fatalf("walk prompt sources: %v", err)
	}

	if len(bashFenceOffenders) > 0 {
		t.Fatalf("bash fences in markdown/templates:\n%s", strings.Join(bashFenceOffenders, "\n"))
	}
	if len(promptFenceOffenders) > 0 {
		t.Fatalf("copyable fences in prompt files:\n%s", strings.Join(promptFenceOffenders, "\n"))
	}
}

func findCopyableFenceOffenders(root string) ([]string, []string, error) {
	var bashFenceOffenders []string
	var promptFenceOffenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".md" && ext != ".tpl" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		text := string(data)
		if strings.Contains(text, "```bash") {
			bashFenceOffenders = append(bashFenceOffenders, rel)
		}
		if isPromptSource(rel) && strings.Contains(text, "```") {
			promptFenceOffenders = append(promptFenceOffenders, rel)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return bashFenceOffenders, promptFenceOffenders, nil
}

func isPromptSource(rel string) bool {
	if strings.HasPrefix(rel, "agents/") {
		return true
	}
	return strings.HasPrefix(rel, "templates/") && filepath.Base(rel) != "README.md"
}
