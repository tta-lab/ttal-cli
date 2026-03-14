package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

const osDarwin = "darwin"

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Bidirectional agent communication daemon",
	Long: `Run the ttal daemon — manages Telegram polling, HTTP server,
JSONL watching, and worker cleanup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.Run()
	},
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install daemon launchd plist and config template",
	Long: `Install the ttal daemon as a launchd service and create a config template.

Creates:
  ~/.config/ttal/config.toml   — config template (edit before starting)
  ~/Library/LaunchAgents/io.guion.ttal.daemon.plist

Also removes the old io.guion.ttal.poll-completion plist if present.

Example:
  ttal daemon install
  # Edit ~/.config/ttal/config.toml
  # ttal daemon status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != osDarwin {
			return fmt.Errorf("daemon install is macOS-only (launchd)")
		}
		return daemon.Install()
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove daemon launchd plist",
	Long:  `Remove the ttal daemon launchd service. Config and logs are preserved.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != osDarwin {
			return fmt.Errorf("daemon uninstall is macOS-only (launchd)")
		}
		return daemon.Uninstall()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the daemon is running",
	Long:  `Check whether the ttal daemon is running by inspecting the pid file and socket.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		running, pid, err := daemon.IsRunning()
		if err != nil {
			return err
		}

		sockPath, _ := daemon.SocketPath()

		if running {
			fmt.Printf("Daemon: running (pid=%d)\n", pid)
			fmt.Printf("Socket: %s (HTTP)\n", sockPath)
		} else {
			fmt.Println("Daemon: not running")
			if pid != 0 {
				fmt.Printf("  Stale pid file (pid=%d, process not found)\n", pid)
			}
		}
		return nil
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon via launchd",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != osDarwin {
			return fmt.Errorf("daemon start is macOS-only (launchd)")
		}
		return daemon.Start()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon via launchd",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != osDarwin {
			return fmt.Errorf("daemon stop is macOS-only (launchd)")
		}
		return daemon.Stop()
	},
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the daemon via launchd",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != osDarwin {
			return fmt.Errorf("daemon restart is macOS-only (launchd)")
		}
		return daemon.Restart()
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
}
