package cmd

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// semverRe validates semver tags: v1.0.0, v1.0.0-rc.1, v1.0.0-guion.1, v1.0.0+build.123
// Pre-release segments are dot-separated alphanumeric (hyphens NOT allowed within segments
// to keep validation simple — use dots as separators: v1.0.0-rc.1 not v1.0.0-rc-1).
var semverRe = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`)

var tagCmd = &cobra.Command{
	Use:   "tag <version> --project <alias>",
	Short: "Create and push a git tag via daemon (no credentials needed in worker)",
	Long: `Creates a lightweight git tag and pushes it to origin through the daemon.
The daemon injects credentials — workers don't need tokens in their environment.

The tag must be a valid semver version prefixed with 'v' (e.g. v1.0.0, v2.1.0-rc.1).

Examples:
  ttal tag v1.0.0 --project ttal-cli
  ttal tag v2.1.0-rc.1 --project fn-agent
  ttal tag v1.1.1-guion.1 --project fn-cli`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tag := args[0]
		if !semverRe.MatchString(tag) {
			return fmt.Errorf("invalid semver tag %q — expected format: v1.0.0, v2.1.0-rc.1", tag)
		}

		projectAlias, _ := cmd.Flags().GetString("project")
		projectPath, err := project.GetProjectPath(projectAlias)
		if err != nil {
			return err
		}

		resp, err := daemon.GitTag(daemon.GitTagRequest{
			WorkDir: projectPath,
			Tag:     tag,
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
	tagCmd.Flags().StringP("project", "p", "", "project alias (required)")
	_ = tagCmd.MarkFlagRequired("project")
	rootCmd.AddCommand(tagCmd)
}
