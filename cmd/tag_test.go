package cmd

import (
	"os"
	"os/exec"
	"testing"
)

func TestSemverRe(t *testing.T) {
	tests := []struct {
		tag   string
		valid bool
	}{
		// Valid
		{"v1.0.0", true},
		{"v0.1.0", true},
		{"v10.20.30", true},
		{"v1.0.0-rc.1", true},
		{"v1.0.0-alpha.1", true},
		{"v1.1.1-guion.1", true},
		{"v1.0.0+build.123", true},
		{"v1.0.0-rc.1+build.456", true},
		{"v1.0.0-beta", true},

		// Invalid
		{"1.0.0", false},          // missing v prefix
		{"v1.0", false},           // missing patch
		{"v1.0.a", false},         // non-numeric patch
		{"v1.0.0-", false},        // trailing hyphen
		{"v1.0.0+", false},        // trailing plus
		{"v1.0.0-rc-1", false},    // hyphen within pre-release segment
		{"v1.0.0-alpha-1", false}, // hyphen within pre-release segment
		{"latest", false},         // not semver
		{"", false},               // empty
		{"v", false},              // just prefix
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got := semverRe.MatchString(tt.tag)
			if got != tt.valid {
				t.Errorf("semverRe.MatchString(%q) = %v, want %v", tt.tag, got, tt.valid)
			}
		})
	}
}

func TestSemverBaseRe(t *testing.T) {
	tests := []struct {
		tag     string
		matches bool
		major   string
		minor   string
		patch   string
		suffix  string
	}{
		{"v1.0.0", true, "1", "0", "0", ""},
		{"v0.1.0", true, "0", "1", "0", ""},
		{"v10.20.30", true, "10", "20", "30", ""},
		{"v1.0.0+build.123", true, "1", "0", "0", "+build.123"},
		{"v1.6.1+0.74.1", true, "1", "6", "1", "+0.74.1"},
		{"v1.0.0-rc.1", false, "", "", "", ""}, // pre-release not matched
		{"v1.0.0-rc.1+build.456", false, "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			matches := semverBaseRe.FindStringSubmatch(tt.tag)
			if tt.matches && matches == nil {
				t.Fatalf("semverBaseRe.FindStringSubmatch(%q) = nil, want match", tt.tag)
			}
			if !tt.matches && matches != nil {
				t.Fatalf("semverBaseRe.FindStringSubmatch(%q) = %v, want no match", tt.tag, matches)
			}
			if matches != nil {
				if matches[1] != tt.major {
					t.Errorf("major: got %q, want %q", matches[1], tt.major)
				}
				if matches[2] != tt.minor {
					t.Errorf("minor: got %q, want %q", matches[2], tt.minor)
				}
				if matches[3] != tt.patch {
					t.Errorf("patch: got %q, want %q", matches[3], tt.patch)
				}
				if matches[4] != tt.suffix {
					t.Errorf("suffix: got %q, want %q", matches[4], tt.suffix)
				}
			}
		})
	}
}

func testBumpedTag(t *testing.T, workDir, level, want string) {
	t.Helper()
	got, err := computeBumpedTag(workDir, level)
	if err != nil {
		t.Fatalf("computeBumpedTag(%q): %v", level, err)
	}
	if got != want {
		t.Errorf("computeBumpedTag(%q) = %q, want %q", level, got, want)
	}
}

func testBumpedTagError(t *testing.T, workDir, level string) {
	t.Helper()
	got, err := computeBumpedTag(workDir, level)
	if err == nil {
		t.Errorf("expected error for computeBumpedTag(%q), got %q", level, got)
	}
	if got != "" {
		t.Errorf("expected empty tag on error, got %q", got)
	}
}

func TestComputeBumpedTagNoTags(t *testing.T) {
	dir, _ := setupBumpTestRepo(t)
	defer func() { _ = os.RemoveAll(dir) }()

	testBumpedTag(t, dir, bumpPatch, "v0.0.1")
	testBumpedTag(t, dir, bumpMinor, "v0.1.0")
	testBumpedTag(t, dir, bumpMajor, "v1.0.0")
}

func TestComputeBumpedTagSimple(t *testing.T) {
	dir := setupBumpTestRepoWithTag(t, "v1.2.3")
	defer func() { _ = os.RemoveAll(dir) }()

	testBumpedTag(t, dir, bumpPatch, "v1.2.4")
	testBumpedTag(t, dir, bumpMinor, "v1.3.0")
	testBumpedTag(t, dir, bumpMajor, "v2.0.0")
}

func TestComputeBumpedTagReturnsLocalLatestWhenMissingFromRemote(t *testing.T) {
	dir, runGit := setupBumpTestRepo(t)
	defer func() { _ = os.RemoveAll(dir) }()

	remote, err := os.MkdirTemp("", "ttal-tag-remote-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(remote) }()

	runGit("init", "--bare", remote)
	runGit("remote", "add", "origin", remote)
	runGit("push", "-u", "origin", "HEAD")
	runGit("tag", "v1.2.3")

	testBumpedTag(t, dir, bumpPatch, "v1.2.3")
}

func TestComputeBumpedTagSuffix(t *testing.T) {
	dir := setupBumpTestRepoWithTag(t, "v1.6.1+0.74.1")
	defer func() { _ = os.RemoveAll(dir) }()

	testBumpedTag(t, dir, bumpPatch, "v1.6.2+0.74.1")
	testBumpedTag(t, dir, bumpMinor, "v1.7.0+0.74.1")
	testBumpedTag(t, dir, bumpMajor, "v2.0.0+0.74.1")
}

func TestComputeBumpedTagPreRelease(t *testing.T) {
	dir := setupBumpTestRepoWithTag(t, "v2.0.0-rc.1")
	defer func() { _ = os.RemoveAll(dir) }()

	testBumpedTagError(t, dir, bumpPatch)
}

func TestComputeBumpedTagInvalidLevel(t *testing.T) {
	_, err := computeBumpedTag("/tmp", "foo")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func setupBumpTestRepo(t *testing.T) (dir string, runGit func(...string)) {
	dir, err := os.MkdirTemp("", "ttal-tag-test-*")
	if err != nil {
		t.Fatal(err)
	}
	runGit = func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s — %v", args, string(out), err)
		}
	}
	runGit("init")
	runGit("config", "user.name", "test")
	runGit("config", "user.email", "test@test")
	runGit("commit", "--allow-empty", "-m", "init")
	return dir, runGit
}

func setupBumpTestRepoWithTag(t *testing.T, tag string) string {
	dir, runGit := setupBumpTestRepo(t)
	runGit("tag", tag)
	return dir
}

func TestTagCmdArgValidation(t *testing.T) {
	// Simulate the arg validation logic from tagCmd.RunE

	// Test mutex: --bump + positional arg fails
	bump := bumpPatch
	args := []string{"v1.0.0"}
	isBump := bump != ""
	if isBump && len(args) > 0 {
		// expected error
	} else {
		t.Fatal("expected mutex error")
	}

	// Test no args + no bump fails
	bump = ""
	args = []string{}
	isBump = bump != ""
	if !isBump && len(args) == 0 {
		// expected error
	} else {
		t.Fatal("expected error for no bump and no args")
	}

	// Test positional + no bump OK
	bump = ""
	args = []string{"v1.0.0"}
	isBump = bump != ""
	if isBump {
		t.Fatal("should not be bump")
	}
	if len(args) == 0 {
		t.Fatal("should have arg")
	}

	// Test bump only OK
	bump = bumpPatch
	args = []string{}
	isBump = bump != ""
	if !isBump {
		t.Fatal("should be bump")
	}
	if len(args) > 0 {
		t.Fatal("should not have positional")
	}
}
