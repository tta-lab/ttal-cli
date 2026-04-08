package cmdexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/project"
)

func TestPolicyForAgent(t *testing.T) {
	tmp := t.TempDir()

	// Create projects.toml with two active and one archived project.
	storePath := filepath.Join(tmp, "projects.toml")
	store := project.NewStore(storePath)

	projectsToml := `[ttal]
name = "TTAL Core"
path = "` + filepath.Join(tmp, "code", "ttal") + `"

[clawd]
name = "Clawd Workspace"
path = "` + filepath.Join(tmp, "code", "clawd") + `"
`
	archivedToml := `[archived]
[archived.old]
name = "Old Archived"
path = "` + filepath.Join(tmp, "archived") + `"
`
	combined := projectsToml + archivedToml
	if err := os.WriteFile(storePath, []byte(combined), 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}

	// Create the active project directories.
	for _, dir := range []string{
		filepath.Join(tmp, "code", "ttal"),
		filepath.Join(tmp, "code", "clawd"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	agentCwd := filepath.Join(tmp, "code", "athena-workspace")
	if err := os.MkdirAll(agentCwd, 0o755); err != nil {
		t.Fatalf("mkdir agent cwd: %v", err)
	}

	paths, ok := PolicyForAgent(store, agentCwd)
	if !ok {
		t.Fatal("expected ok=true for valid agentCwd")
	}

	// Should have 3 paths: agent cwd (rw) + 2 active projects (ro).
	// Archived projects are excluded.
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d: %v", len(paths), paths)
	}

	// First path is agent cwd, read-write.
	if paths[0].Path != agentCwd {
		t.Errorf("paths[0].Path = %q, want %q", paths[0].Path, agentCwd)
	}
	if paths[0].ReadOnly {
		t.Error("paths[0].ReadOnly: expected false (rw), got true")
	}

	// Subsequent paths are read-only.
	for i := 1; i < len(paths); i++ {
		if !paths[i].ReadOnly {
			t.Errorf("paths[%d].ReadOnly: expected true (ro), got false", i)
		}
	}
}

func TestPolicyForAgent_EmptyCwd(t *testing.T) {
	tmp := t.TempDir()
	store := project.NewStore(filepath.Join(tmp, "projects.toml"))

	paths, ok := PolicyForAgent(store, "")
	if ok {
		t.Error("expected ok=false for empty agentCwd")
	}
	if len(paths) != 0 {
		t.Errorf("expected nil paths for empty agentCwd, got %v", paths)
	}
}

func TestPolicyForAgent_AgentCwdOverlapProject(t *testing.T) {
	tmp := t.TempDir()

	// Agent's cwd IS a registered project.
	projectPath := filepath.Join(tmp, "myproject")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	storePath := filepath.Join(tmp, "projects.toml")
	store := project.NewStore(storePath)
	projectsToml := `[myproject]
name = "My Project"
path = "` + projectPath + `"
`
	if err := os.WriteFile(storePath, []byte(projectsToml), 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}

	paths, ok := PolicyForAgent(store, projectPath)
	if !ok {
		t.Fatal("expected ok=true")
	}

	// Should have ONE path: the project (rw wins).
	if len(paths) != 1 {
		t.Errorf("expected 1 path (cwd overlaps project), got %d: %v", len(paths), paths)
	}
	if paths[0].ReadOnly {
		t.Error("paths[0].ReadOnly: expected false (rw), got true")
	}
}

func TestPolicyForAgent_NoProjects(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "projects.toml")
	store := project.NewStore(storePath)

	// Empty projects.toml (no sections).
	if err := os.WriteFile(storePath, []byte("# empty\n"), 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}

	agentCwd := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(agentCwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	paths, ok := PolicyForAgent(store, agentCwd)
	if !ok {
		t.Fatal("expected ok=true")
	}

	// Should have just the agent cwd (rw).
	if len(paths) != 1 {
		t.Errorf("expected 1 path (agent cwd only), got %d: %v", len(paths), paths)
	}
	if paths[0].Path != agentCwd || paths[0].ReadOnly {
		t.Errorf("expected cwd path rw, got %+v", paths[0])
	}
}

func TestPolicyForAgentFS(t *testing.T) {
	tmp := t.TempDir()

	// Create projects.toml in the team root.
	teamRoot := tmp
	storePath := filepath.Join(teamRoot, "projects.toml")
	projectsToml := `[proj]
name = "Proj"
path = "` + filepath.Join(tmp, "proj") + `"
`
	if err := os.WriteFile(storePath, []byte(projectsToml), 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "proj"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	agentCwd := filepath.Join(tmp, "agent-ws")
	if err := os.MkdirAll(agentCwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	paths, ok := PolicyForAgentFS(teamRoot, agentCwd)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Should have 2: agent cwd (rw) + project (ro).
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

func TestAllowedPathType(t *testing.T) {
	// Verify logos.AllowedPath is compatible with our usage.
	var ap logos.AllowedPath
	ap.Path = "/tmp/test"
	ap.ReadOnly = true
	if ap.Path != "/tmp/test" {
		t.Error("AllowedPath.Path not set correctly")
	}
	if !ap.ReadOnly {
		t.Error("AllowedPath.ReadOnly not set correctly")
	}
}
