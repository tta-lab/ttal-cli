package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the ttal chat desktop app",
	Long: `Launch the ttal-gui desktop app (Wails + SvelteKit).

Requires ttal-gui to be installed. Build and install it with:

  cd gui && go build -o ~/go/bin/ttal-gui .

Or install via Homebrew when a formula is available.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		binary, err := findTtalGui()
		if err != nil {
			return err
		}

		proc := exec.Command(binary, args...)
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		return proc.Run()
	},
}

// findTtalGui searches for the ttal-gui binary in PATH and common install locations.
func findTtalGui() (string, error) {
	if p, err := exec.LookPath("ttal-gui"); err == nil {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, "go", "bin", "ttal-gui"),
			filepath.Join(home, ".local", "bin", "ttal-gui"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("ttal-gui not found in PATH — install with: cd gui && go build -o ~/go/bin/ttal-gui")
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
