package reporef

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindClonedRepo_DirNotExist(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "this", "does", "not", "exist")

	_, err := FindClonedRepo("myrepo", nonExistentPath)

	if err == nil {
		t.Fatal("expected error when references path does not exist")
	}
	if !strings.Contains(err.Error(), "myrepo") {
		t.Errorf("error should contain repo name %q, got: %v", "myrepo", err)
	}
}

func TestFindClonedRepo_SingleMatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory structure: github.com/myorg/myrepo
	repoPath := filepath.Join(tempDir, "github.com", "myorg", "myrepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	result, err := FindClonedRepo("myrepo", tempDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != repoPath {
		t.Errorf("expected %q, got %q", repoPath, result)
	}
}

func TestFindClonedRepo_NoMatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create a different repo that doesn't match
	repoPath := filepath.Join(tempDir, "github.com", "otherorg", "otherrepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	_, err := FindClonedRepo("myrepo", tempDir)

	if err == nil {
		t.Fatal("expected error when no matching repo found")
	}
	if !strings.Contains(err.Error(), "org/repo") {
		t.Errorf("error should suggest org/repo format, got: %v", err)
	}
}

func TestFindClonedRepo_MultipleMatches(t *testing.T) {
	tempDir := t.TempDir()

	// Create same repo name under two different orgs
	repo1Path := filepath.Join(tempDir, "github.com", "org1", "myrepo")
	repo2Path := filepath.Join(tempDir, "github.com", "org2", "myrepo")
	if err := os.MkdirAll(repo1Path, 0755); err != nil {
		t.Fatalf("failed to create repo1 directory: %v", err)
	}
	if err := os.MkdirAll(repo2Path, 0755); err != nil {
		t.Fatalf("failed to create repo2 directory: %v", err)
	}

	_, err := FindClonedRepo("myrepo", tempDir)

	if err == nil {
		t.Fatal("expected error when multiple matches found")
	}
	if !strings.Contains(err.Error(), "org1/myrepo") {
		t.Errorf("error should list org1/myrepo, got: %v", err)
	}
	if !strings.Contains(err.Error(), "org2/myrepo") {
		t.Errorf("error should list org2/myrepo, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should mention ambiguity, got: %v", err)
	}
}
