package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

func TestDefaultPipelinesContent_ParsesCleanly(t *testing.T) {
	var raw map[string]interface{}
	if _, err := toml.Decode(defaultPipelinesContent, &raw); err != nil {
		t.Fatalf("defaultPipelinesContent is not valid TOML: %v", err)
	}
}

func TestDefaultPipelinesContent_HasExpectedPipelines(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "pipelines.toml"), []byte(defaultPipelinesContent), 0o644)
	if err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg, err := pipeline.Load(dir)
	if err != nil {
		t.Fatalf("pipeline.Load: %v", err)
	}

	for _, name := range []string{"standard", "bugfix", "hotfix"} {
		p, ok := cfg.Pipelines[name]
		if !ok {
			t.Errorf("pipeline %q not found", name)
			continue
		}
		if len(p.Stages) == 0 {
			t.Errorf("pipeline %q has no stages", name)
		}
		if len(p.Tags) == 0 {
			t.Errorf("pipeline %q has no tags", name)
		}
	}
}

func TestDefaultPipelinesContent_ReviewerNames(t *testing.T) {
	for _, reviewer := range []string{"plan-review-lead", "pr-review-lead"} {
		if !strings.Contains(defaultPipelinesContent, reviewer) {
			t.Errorf("expected reviewer %q in default pipelines content", reviewer)
		}
	}
}
