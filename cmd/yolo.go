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
	Long: `Launch a coding agent in yolo mode (skip all permission prompts).

Supported runtimes: cc (Claude Code), codex.
For human use only - starts the agent with full permissions enabled.`,
}

var yoloCcCmd = &cobra.Command{
	Use:   "cc",
	Short: "Launch Claude Code in yolo mode",
	Long: `Launch Claude Code with --dangerously-skip-permissions.

Example:
  ttal yolo cc              # Start in current directory with sonnet model
  ttal yolo cc --model sonnet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("claude"); err != nil {
			return fmt.Errorf("claude not found in PATH — install Claude Code first")
		}
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		fmt.Printf("Starting Claude Code in yolo mode...\n")
		fmt.Printf("  Directory: %s\n", workDir)
		fmt.Printf("  Model: %s\n", yoloModel)
		fmt.Println()

		ccCmd := exec.Command("claude", "--dangerously-skip-permissions", "--model", yoloModel)
		return runYolo(ccCmd, "claude")
	},
}

var yoloCxCmd = &cobra.Command{
	Use:   "codex",
	Short: "Launch Codex in yolo mode",
	Long: `Launch Codex with --yolo flag.

Example:
  ttal yolo codex           # Start in current directory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("codex"); err != nil {
			return fmt.Errorf("codex not found in PATH — install Codex first")
		}
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		fmt.Printf("Starting Codex in yolo mode...\n")
		fmt.Printf("  Directory: %s\n", workDir)
		fmt.Println()

		cxCmd := exec.Command("codex", "--yolo")
		return runYolo(cxCmd, "codex")
	},
}

// runYolo executes the command with stdio wired for interactive TUI.
// os.Exit is used to propagate the child's exit code directly, bypassing
// cobra's cleanup — safe here since PersistentPostRunE is a no-op.
func runYolo(cmd *exec.Cmd, name string) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run %s: %w", name, err)
	}
	return nil
}
func init() {
	yoloCcCmd.Flags().StringVarP(&yoloModel, "model", "m", "sonnet", "Model to use (haiku, sonnet, opus)")
	yoloCmd.AddCommand(yoloCcCmd)
	yoloCmd.AddCommand(yoloCxCmd)
	rootCmd.AddCommand(yoloCmd)
}
