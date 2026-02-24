package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var yoloModel string

var yoloCmd = &cobra.Command{
	Use:   "yolo",
	Short: "Launch coding agent in yolo mode",
	Long: `Launch Claude Code or OpenCode in yolo mode (skip all permission prompts).

For human use only - starts the agent with full permissions enabled.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var yoloCcCmd = &cobra.Command{
	Use:   "cc",
	Short: "Launch Claude Code in yolo mode",
	Long: `Launch Claude Code with --dangerously-skip-permissions.

Example:
  ttal yolo cc              # Start in current directory with opus model
  ttal yolo cc --model sonnet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, _ := os.Getwd()
		fmt.Printf("Starting Claude Code in yolo mode...\n")
		fmt.Printf("  Directory: %s\n", workDir)
		fmt.Printf("  Model: %s\n", yoloModel)
		fmt.Println()

		ccCmd := exec.Command("claude", "--dangerously-skip-permissions", "--model", yoloModel)
		ccCmd.Stdin = os.Stdin
		ccCmd.Stdout = os.Stdout
		ccCmd.Stderr = os.Stderr

		if err := ccCmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run claude: %w", err)
		}
		return nil
	},
}

var yoloOcCmd = &cobra.Command{
	Use:   "oc",
	Short: "Launch OpenCode in yolo mode",
	Long: `Launch OpenCode with OPENCODE_PERMISSION set to allow all operations.

Example:
  ttal yolo oc              # Start in current directory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, _ := os.Getwd()
		fmt.Printf("Starting OpenCode in yolo mode...\n")
		fmt.Printf("  Directory: %s\n", workDir)
		fmt.Println()

		ocCmd := exec.Command("opencode")
		ocCmd.Stdin = os.Stdin
		ocCmd.Stdout = os.Stdout
		ocCmd.Stderr = os.Stderr
		ocCmd.Env = append(os.Environ(),
			`OPENCODE_PERMISSION={"bash":"allow","edit":"allow","read":"allow","write":"allow","question":"allow"}`)

		if err := ocCmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run opencode: %w", err)
		}
		return nil
	},
}

func init() {
	yoloCcCmd.Flags().StringVarP(&yoloModel, "model", "m", "opus", "Model to use (opus, sonnet)")
	yoloCmd.AddCommand(yoloCcCmd)
	yoloCmd.AddCommand(yoloOcCmd)
	rootCmd.AddCommand(yoloCmd)
}
