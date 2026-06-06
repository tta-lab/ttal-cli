package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
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
	Use:   "jump <alias|org/repo>",
	Short: "Print path to a project or cloned repo directory",
	Long: `Print the filesystem path for a project alias or cloned repo name.

Designed to be wrapped in a shell function that performs the cd:

  zsh/bash — add to ~/.zshrc or ~/.bashrc:
    eval "$(ttal jump --init zsh)"

  fish — add to ~/.config/fish/config.fish:
    ttal jump --init fish | source

Then use: t <alias> or t <org/repo>   (e.g. t ttal, t crush, t charmbracelet/crush)

Delegates to the project CLI binary for all resolution logic.`,
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

	out, err := exec.Command("project", "jump", target).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("project jump: %w", err)
	}

	fmt.Print(string(out))
	return nil
}

func init() {
	jumpCmd.Flags().StringVar(&jumpFlags.initShell, "init", "",
		"Print shell function for the given shell (zsh, bash, fish)")
	rootCmd.AddCommand(jumpCmd)
}
