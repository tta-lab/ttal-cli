package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/reporef"
)

// Shell functions installed via --init. command ttal prevents alias recursion.
// Fish: "or return 1" MUST be on its own line — in Fish, "or" chains the exit
// code of the command substitution inside set, not set itself.
const jumpFuncZsh = `t() {
  local dir
  dir="$(command ttal jump "$@")" && cd "$dir"
}
`

const jumpFuncFish = `function t
    set -l dir (command ttal jump $argv)
    or return 1
    cd $dir
end
`

var jumpFlags struct {
	initShell string
}

var jumpCmd = &cobra.Command{
	Use:   "jump <alias|reponame>",
	Short: "Print path to a project or cloned repo directory",
	Long: `Print the filesystem path for a project alias or cloned repo name.

Designed to be wrapped in a shell function that performs the cd:

  zsh/bash — add to ~/.zshrc or ~/.bashrc:
    eval "$(ttal jump --init zsh)"

  fish — add to ~/.config/fish/config.fish:
    ttal jump --init fish | source

Then use: t <alias>   (e.g. t ttal, t fn-cli)

Resolution order:
  1. Exact project alias match (projects.toml)
  2. Bare repo name in ~/.ttal/references/ (already-cloned repos)`,
	Args: cobra.ArbitraryArgs,
	RunE: runJump,
}

func runJump(cmd *cobra.Command, args []string) error {
	if jumpFlags.initShell != "" {
		switch jumpFlags.initShell {
		case "zsh", "bash":
			fmt.Print(jumpFuncZsh)
			return nil
		case "fish":
			fmt.Print(jumpFuncFish)
			return nil
		default:
			return fmt.Errorf("unsupported shell %q (supported: zsh, bash, fish)", jumpFlags.initShell)
		}
	}

	if len(args) != 1 {
		return fmt.Errorf("usage: ttal jump <alias|reponame>\n\nTo install the shell function: ttal jump --init zsh")
	}
	target := args[0]

	// 1. Try project alias (exact match).
	projPath, err := project.GetProjectPath(target)
	if err == nil {
		fmt.Println(projPath)
		return nil
	}

	// 2. Try bare repo name in references directory.
	// AskReferencesPath handles the default (~/.ttal/references/) on a zero-value Config,
	// so this works even when config.toml doesn't exist.
	cfg, cfgErr := config.Load()
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: config load failed, using default references path: %v\n", cfgErr)
	}
	refsPath := cfg.AskReferencesPath()

	repoPath, repoErr := reporef.FindClonedRepo(target, refsPath)
	if repoErr == nil {
		fmt.Println(repoPath)
		return nil
	}

	// Surface repo lookup failure to help debug references path issues.
	fmt.Fprintf(cmd.ErrOrStderr(), "note: repo lookup also failed: %v\n", repoErr)

	// Return the project error — more actionable than the repo error.
	return err
}

func init() {
	jumpCmd.Flags().StringVar(&jumpFlags.initShell, "init", "",
		"Print shell function for the given shell (zsh, bash, fish)")
	rootCmd.AddCommand(jumpCmd)
}
