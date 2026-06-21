package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// semverRe validates semver tags: v1.0.0, v1.0.0-rc.1, v1.0.0-guion.1, v1.0.0+build.123
// Pre-release segments are dot-separated alphanumeric (hyphens NOT allowed within segments
// to keep validation simple — use dots as separators: v1.0.0-rc.1 not v1.0.0-rc-1).
var semverRe = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`)

// semverBaseRe captures vMAJOR.MINOR.PATCH and optional +suffix from a tag.
// Groups: 1=full base (v1.2.3), 2=MAJOR, 3=MINOR, 4=PATCH, 5=+suffix (including +).
var semverBaseRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(\+[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`)

var tagCmd = &cobra.Command{
	Use:   "tag [<version> | --bump <major|minor|patch>]",
	Short: "Create and push a git tag via daemon (no credentials needed in worker)",
	Long: `Creates a lightweight git tag and pushes it to origin through the daemon.
The daemon injects credentials — workers don't need tokens in their environment.

Resolves the project from the current working directory. No --project flag needed.

With --bump, automatically bumps the largest existing version tag in the repo.
Existing +suffix (e.g. +0.74.1) is preserved on bump.

With a positional version argument, tags that exact version directly.

Examples:
  ttal tag --bump patch          # v1.2.3 → v1.2.4
  ttal tag --bump minor          # v1.2.3 → v1.3.0
  ttal tag --bump major          # v1.2.3 → v2.0.0
  ttal tag v2.0.0                # explicit version
  ttal tag v1.6.1+0.74.1         # explicit with +suffix`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}

		projectAlias := project.ResolveProjectAlias(cwd)
		if projectAlias == "" {
			return fmt.Errorf("current directory %q is not under a registered ttal project", cwd)
		}
		projectPath, err := project.GetProjectPath(projectAlias)
		if err != nil {
			return err
		}

		bump, _ := cmd.Flags().GetString("bump")
		isBump := bump != ""

		if isBump && len(args) > 0 {
			return fmt.Errorf("--bump and a positional version are mutually exclusive")
		}

		var tag string

		if isBump {
			switch bump {
			case bumpMajor, bumpMinor, bumpPatch:
			default:
				return fmt.Errorf("invalid --bump value %q — must be major, minor, or patch", bump)
			}
			tag, err = computeBumpedTag(projectPath, bump)
			if err != nil {
				return err
			}
		} else {
			if len(args) == 0 {
				return fmt.Errorf("either a version argument or --bump is required")
			}
			tag = args[0]
			if !semverRe.MatchString(tag) {
				return fmt.Errorf("invalid semver tag %q — expected format: v1.0.0, v2.1.0-rc.1", tag)
			}
		}

		resp, err := daemon.GitTag(daemon.GitTagRequest{
			WorkDir:      projectPath,
			Tag:          tag,
			ProjectAlias: projectAlias,
		})
		if err != nil {
			return fmt.Errorf("tag failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("tag failed: %s", resp.Error)
		}

		fmt.Printf("Tagged %s → pushed to origin\n", tag)
		return nil
	},
}

func init() {
	tagCmd.Flags().String("bump", "", "bump version: major, minor, or patch")
	rootCmd.AddCommand(tagCmd)
}

// latestTag returns the largest semver tag in the repo, or "" if none exist.
func latestTag(workDir string) (string, error) {
	cmd := exec.Command("git", "-C", workDir, "tag", "--sort=-version:refname")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git tag: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", nil
	}
	return lines[0], nil
}

// shouldBumpLatestTag reports whether --bump should create a new tag from latest.
// Repos without an origin keep the old local-only bump behavior.
func shouldBumpLatestTag(workDir, tag string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := exec.CommandContext(ctx, "git", "-C", workDir, "remote", "get-url", "origin").Run(); err != nil {
		return true, nil
	}

	ref := "refs/tags/" + tag
	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "ls-remote", "--exit-code", "--tags", "origin", ref)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return false, nil
		}
		return false, fmt.Errorf("check remote tag %q: %w", tag, err)
	}
	return true, nil
}

const (
	initialMajor = "v1.0.0"
	initialMinor = "v0.1.0"
	initialPatch = "v0.0.1"

	bumpMajor = "major"
	bumpMinor = "minor"
	bumpPatch = "patch"
)

// computeBumpedTag finds the largest tag, bumps the specified level, and returns the new tag.
// The +suffix from the latest tag is preserved.
func computeBumpedTag(workDir, level string) (string, error) {
	latest, err := latestTag(workDir)
	if err != nil {
		return "", err
	}
	if latest == "" {
		switch level {
		case bumpMajor:
			return initialMajor, nil
		case bumpMinor:
			return initialMinor, nil
		default:
			return initialPatch, nil
		}
	}

	shouldBump, err := shouldBumpLatestTag(workDir, latest)
	if err != nil {
		return "", err
	}
	if !shouldBump {
		return latest, nil
	}

	matches := semverBaseRe.FindStringSubmatch(latest)
	if matches == nil {
		return "", fmt.Errorf(
			"latest tag %q is not a plain semver with optional +suffix (has pre-release: %s)",
			latest, latest)
	}

	maj, _ := strconv.Atoi(matches[1])
	min, _ := strconv.Atoi(matches[2])
	pat, _ := strconv.Atoi(matches[3])
	suffix := matches[4] // includes leading +, or "" if absent

	switch level {
	case bumpMajor:
		maj++
		min = 0
		pat = 0
	case bumpMinor:
		min++
		pat = 0
	default:
		pat++
	}

	return fmt.Sprintf("v%d.%d.%d%s", maj, min, pat, suffix), nil
}
