package daemon

import (
	"strings"
	"testing"
)

func TestPlistContent(t *testing.T) {
	plist := buildPlistContent(
		"io.guion.ttal.daemon",
		"/usr/local/bin/ttal",
		"/Users/test/.ttal",
		"/Users/test",
	)

	// Must contain required fields
	if !strings.Contains(plist, "<string>io.guion.ttal.daemon</string>") {
		t.Error("plist missing label")
	}
	if !strings.Contains(plist, "<string>/usr/local/bin/ttal</string>") {
		t.Error("plist missing ttal binary path")
	}
	if !strings.Contains(plist, "<key>PATH</key>") {
		t.Error("plist missing PATH key")
	}
	if !strings.Contains(plist, "/Users/test/.local/bin") {
		t.Error("plist PATH missing home-based bin dirs")
	}

	// Must NOT contain credential keys — prevents future re-introduction of credential baking
	if strings.Contains(plist, "FORGEJO_TOKEN") {
		t.Error("plist must not contain FORGEJO_TOKEN — credentials are loaded at runtime from .env")
	}
	if strings.Contains(plist, "FORGEJO_URL") {
		t.Error("plist must not contain FORGEJO_URL — credentials are loaded at runtime from .env")
	}
	if strings.Contains(plist, "GITHUB_TOKEN") {
		t.Error("plist must not contain GITHUB_TOKEN — credentials are loaded at runtime from .env")
	}
}

func TestPlistContent_DataDirUsed(t *testing.T) {
	plist := buildPlistContent("label", "/bin/ttal", "/custom/datadir", "/home/user")

	if !strings.Contains(plist, "/custom/datadir/daemon.log") {
		t.Errorf("plist StandardOutPath should use dataDir, got:\n%s", plist)
	}
	// Both stdout and stderr should point to the same dataDir log file.
	count := strings.Count(plist, "/custom/datadir/daemon.log")
	if count < 2 {
		t.Errorf("expected both StandardOutPath and StandardErrorPath to use dataDir, found %d occurrences", count)
	}
}
