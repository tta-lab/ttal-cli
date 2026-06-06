package daemon

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/project"
)

func testGetProject(t *testing.T) func(string) (*project.Project, error) {
	t.Helper()
	return func(alias string) (*project.Project, error) {
		return &project.Project{
			Alias:        alias,
			Name:         "Test",
			Path:         "/test/path",
			K8sApp:       "testapp",
			K8sNamespace: "testns",
		}, nil
	}
}

func TestBuildKubectlArgs(t *testing.T) {
	args := buildKubectlArgs("myapp", "myns", "myctx", 50, "1h")
	if len(args) < 4 {
		t.Fatalf("expected at least 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "logs" {
		t.Errorf("expected 'logs', got %q", args[0])
	}
	// Check app label
	found := false
	for i, a := range args {
		if a == "-l" && i+1 < len(args) && args[i+1] == "app.kubernetes.io/name=myapp" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected app label 'app.kubernetes.io/name=myapp' in args: %v", args)
	}
}

func TestIsNamespaceAllowed(t *testing.T) {
	if !isNamespaceAllowed("a", []string{"a", "b"}) {
		t.Error("expected 'a' to be allowed")
	}
	if isNamespaceAllowed("c", []string{"a", "b"}) {
		t.Error("expected 'c' to not be allowed")
	}
}
