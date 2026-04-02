package worker

import (
	"testing"
)

const (
	testProjectAlias       = "myproj"
	testProjectPath        = "/projects/myproj"
	testWorktreeBranchUUID = "abcd1234"
	testWorktreePath       = "/fake/worktrees/" + testWorktreeBranchUUID + "-" + testProjectAlias
)

// contains reports whether substr is within s (simple helper for test stubs).
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring is a simple substring search.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// stubBranchName replaces branchNameFn for testing.
func stubBranchName(t *testing.T, fn func(dir string) string) func() {
	t.Helper()
	orig := branchNameFn
	branchNameFn = fn
	return func() { branchNameFn = orig }
}

// stubResolveProjectPath replaces resolveProjectPathFn for testing.
func stubResolveProjectPath(t *testing.T, fn func(alias string) string) func() {
	t.Helper()
	orig := resolveProjectPathFn
	resolveProjectPathFn = fn
	return func() { resolveProjectPathFn = orig }
}

// TestCurrentBranch_WorktreePriority verifies worktree is checked first.
func TestCurrentBranch_WorktreePriority(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		return testProjectPath // shouldn't be used
	})
	defer restoreProj()

	worktreeCalled := false
	restore := stubBranchName(t, func(dir string) string {
		// Simulate worktree has a branch (any worktree path for this UUID/alias)
		if contains(dir, testWorktreeBranchUUID+"-"+testProjectAlias) {
			worktreeCalled = true
			return "feature-123"
		}
		// Project path would also have a branch but shouldn't be reached
		if dir == testProjectPath {
			return "should-not-be-used"
		}
		return ""
	})
	defer restore()

	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", testProjectAlias, "/other")
	if got != "feature-123" {
		t.Errorf("CurrentBranch() = %q, want %q (worktree priority)", got, "feature-123")
	}
	if !worktreeCalled {
		t.Error("branchNameFn should be called for worktree path")
	}
}

// TestCurrentBranch_FallsBackToProjectPath verifies project path is tried when worktree has no branch.
func TestCurrentBranch_FallsBackToProjectPath(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		if alias == testProjectAlias {
			return testProjectPath
		}
		return ""
	})
	defer restoreProj()

	restore := stubBranchName(t, func(dir string) string {
		if dir == testWorktreePath {
			return "" // worktree has no branch
		}
		if dir == testProjectPath {
			return "develop" // project has branch
		}
		return ""
	})
	defer restore()

	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", testProjectAlias, "")
	if got != "develop" {
		t.Errorf("CurrentBranch() = %q, want %q (project path fallback)", got, "develop")
	}
}

// TestCurrentBranch_FallsBackToWorkDir verifies workDir is tried when project path fails.
func TestCurrentBranch_FallsBackToWorkDir(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		if alias == testProjectAlias {
			return testProjectPath
		}
		return ""
	})
	defer restoreProj()

	restore := stubBranchName(t, func(dir string) string {
		if dir == testWorktreePath {
			return "" // worktree has no branch
		}
		if dir == testProjectPath {
			return "" // project has no branch
		}
		if dir == "/cwd" {
			return "local-branch" // workDir has branch
		}
		return ""
	})
	defer restore()

	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", testProjectAlias, "/cwd")
	if got != "local-branch" {
		t.Errorf("CurrentBranch() = %q, want %q (workDir fallback)", got, "local-branch")
	}
}

// TestCurrentBranch_AllSourcesEmpty verifies empty string when all sources fail.
func TestCurrentBranch_AllSourcesEmpty(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		return "" // no project path
	})
	defer restoreProj()

	restore := stubBranchName(t, func(dir string) string {
		return "" // nothing has a branch
	})
	defer restore()

	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", testProjectAlias, "/cwd")
	if got != "" {
		t.Errorf("CurrentBranch() = %q, want empty string", got)
	}
}

// TestCurrentBranch_ShortUUIDSkipsWorktree verifies UUID < 8 chars skips worktree.
func TestCurrentBranch_ShortUUIDSkipsWorktree(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		if alias == testProjectAlias {
			return testProjectPath
		}
		return ""
	})
	defer restoreProj()

	worktreeCalled := false
	restore := stubBranchName(t, func(dir string) string {
		// Should never be called for worktree with short UUID
		if dir == "/fake/worktrees/abcd-"+testProjectAlias {
			worktreeCalled = true
			return "worktree-branch"
		}
		if dir == testProjectPath {
			return "short-uuid-branch"
		}
		return ""
	})
	defer restore()

	got := CurrentBranch("abcd", testProjectAlias, "")
	if got != "short-uuid-branch" {
		t.Errorf("CurrentBranch(shortUUID) = %q, want %q", got, "short-uuid-branch")
	}
	if worktreeCalled {
		t.Error("BranchName should not be called for worktree with short UUID")
	}
}

// TestCurrentBranch_EmptyProjectAliasSkipsProjectPath verifies empty alias skips project path.
func TestCurrentBranch_EmptyProjectAliasSkipsProjectPath(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		t.Error("resolveProjectPathFn should not be called when alias is empty")
		return "/should-not-be-used"
	})
	defer restoreProj()

	worktreeCalled := false
	restore := stubBranchName(t, func(dir string) string {
		// With empty alias, worktree path ends with a hyphen
		if contains(dir, testWorktreeBranchUUID+"-") && !contains(dir, testWorktreeBranchUUID+"-0") {
			worktreeCalled = true
			return "from-worktree"
		}
		return ""
	})
	defer restore()

	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", "", "")
	if got != "from-worktree" {
		t.Errorf("CurrentBranch() = %q, want %q (worktree with empty alias)", got, "from-worktree")
	}
	if !worktreeCalled {
		t.Error("branchNameFn should be called for worktree path even with empty alias")
	}
}

// TestCurrentBranch_EmptyWorkDirSkipsWorkDirCheck verifies empty workDir skips that check.
func TestCurrentBranch_EmptyWorkDirSkipsWorkDirCheck(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		if alias == testProjectAlias {
			return testProjectPath
		}
		return ""
	})
	defer restoreProj()

	workDirCalled := false
	restore := stubBranchName(t, func(dir string) string {
		if dir == testWorktreePath {
			return "" // no worktree branch
		}
		if dir == testProjectPath {
			return "project-branch"
		}
		if dir == "" {
			t.Error("BranchName should not be called for empty workDir")
			return ""
		}
		return ""
	})
	defer restore()

	// Note: empty workDir is passed as ""
	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", testProjectAlias, "")
	if got != "project-branch" {
		t.Errorf("CurrentBranch() = %q, want %q (project path when workDir empty)", got, "project-branch")
	}
	_ = workDirCalled // suppress unused warning
}

// TestCurrentBranch_ProjectPathCalledWithCorrectPath verifies the project path is constructed correctly.
func TestCurrentBranch_ProjectPathCalledWithCorrectPath(t *testing.T) {
	restoreProj := stubResolveProjectPath(t, func(alias string) string {
		if alias == testProjectAlias {
			return "/the/project/path"
		}
		return ""
	})
	defer restoreProj()

	projectPathCalled := false
	restore := stubBranchName(t, func(dir string) string {
		if dir == testWorktreePath {
			return "" // no worktree branch
		}
		if dir == "/the/project/path" {
			projectPathCalled = true
			return "main"
		}
		return ""
	})
	defer restore()

	got := CurrentBranch("abcd1234-0000-0000-0000-000000000000", testProjectAlias, "")
	if got != "main" {
		t.Errorf("CurrentBranch() = %q, want %q", got, "main")
	}
	if !projectPathCalled {
		t.Error("BranchName should be called with resolved project path")
	}
}
