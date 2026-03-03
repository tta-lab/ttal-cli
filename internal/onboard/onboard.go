package onboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/doctor"
	"github.com/tta-lab/ttal-cli/internal/scaffold"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// Run executes the full onboard flow.
func Run(workspace, scaffoldName string) error {
	workspace, err := expandPath(workspace)
	if err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	fmt.Println("Welcome to ttal — let's get you set up.")
	fmt.Println()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Prerequisites", installPrerequisites},
		{"Scaffold setup", func() error { return setupScaffold(workspace, scaffoldName) }},
		{"Taskwarrior & config", setupTaskwarriorAndConfig},
		{"Registering agents", func() error { return registerAgents(self, workspace) }},
		{"Daemon & worker hooks", installDaemonAndWorker},
		{"Verify", verify},
	}

	var failures []string
	for i, step := range steps {
		fmt.Printf("Step %d/%d: %s\n", i+1, len(steps), step.name)
		if err := step.fn(); err != nil {
			fmt.Printf("  ✗ %v\n", err)
			failures = append(failures, step.name)
		}
		fmt.Println()
	}

	printNextSteps()

	if len(failures) > 0 {
		return fmt.Errorf("onboard completed with errors in: %s", strings.Join(failures, ", "))
	}
	return nil
}

// --- Step 1: Prerequisites ---

func installPrerequisites() error {
	if _, err := exec.LookPath("brew"); err != nil {
		fmt.Println("  ! Homebrew not found. Install from https://brew.sh")
		fmt.Println("    Then re-run: ttal onboard")
		return fmt.Errorf("homebrew required")
	}

	tools := []struct {
		formula string
		bin     string
	}{
		{"tmux", "tmux"},
		{"task", "task"},
		{"zellij", "zellij"},
		{"ffmpeg", "ffmpeg"},
	}

	var failed []string
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.bin); err == nil {
			fmt.Printf("  %s already installed\n", tool.formula)
			continue
		}
		fmt.Printf("  Installing %s...", tool.formula)
		out, err := exec.Command("brew", "install", tool.formula).CombinedOutput()
		if err != nil {
			fmt.Printf(" failed\n")
			// Show first line of error output for diagnosis
			if lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2); len(lines) > 0 && lines[0] != "" {
				fmt.Printf("    %s\n", lines[0])
			}
			failed = append(failed, tool.formula)
			continue
		}
		fmt.Println(" done")
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to install: %s", strings.Join(failed, ", "))
	}
	return nil
}

// --- Step 2: Scaffold setup ---

func setupScaffold(workspace, scaffoldName string) error {
	info, err := os.Stat(workspace)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot access %s: %w", workspace, err)
	}
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", workspace)
		}
		entries, _ := os.ReadDir(workspace)
		if len(entries) > 0 {
			fmt.Printf("  %s already exists, skipping scaffold\n", workspace)
			scanAndPrintAgents(workspace)
			return nil
		}
	}

	fmt.Print("  Fetching templates...")
	cacheDir, err := scaffold.EnsureCache()
	if err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("fetch templates: %w", err)
	}
	fmt.Println(" done")

	fmt.Printf("  Applying %s scaffold to %s...", scaffoldName, workspace)
	if err := scaffold.Apply(cacheDir, scaffoldName, workspace); err != nil {
		fmt.Println(" failed")
		return err
	}
	fmt.Println(" done")

	scanAndPrintAgents(workspace)
	return nil
}

func scanAndPrintAgents(workspace string) {
	dirs, err := findAgentDirs(workspace)
	if err != nil {
		fmt.Printf("  ! Could not scan for agents: %v\n", err)
		return
	}
	printAgentDirs(dirs)
}

// --- Step 3: Taskwarrior & config ---

func setupTaskwarriorAndConfig() error {
	// Reuse doctor --fix for taskwarrior UDAs and config template
	report := doctor.Run(true)

	// Report relevant results and count errors
	var errs int
	for _, section := range report.Sections {
		switch section.Name {
		case "Config", "Taskwarrior UDAs":
			for _, check := range section.Checks {
				switch check.Level {
				case doctor.LevelOK:
					fmt.Printf("  %s\n", check.Message)
				case doctor.LevelWarn:
					fmt.Printf("  ! %s\n", check.Message)
				case doctor.LevelError:
					fmt.Printf("  ✗ %s\n", check.Message)
					errs++
				}
			}
		}
	}
	if errs > 0 {
		return fmt.Errorf("%d config/taskwarrior check(s) failed", errs)
	}
	return nil
}

// --- Step 4: Register agents ---

func registerAgents(self, workspace string) error {
	dirs, err := findAgentDirs(workspace)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		fmt.Println("  No agent directories found (expected dirs with CLAUDE.md)")
		return nil
	}

	var failed []string
	for _, dir := range dirs {
		name := filepath.Base(dir)
		cmd := exec.Command(self, "agent", "add", name, "--path", dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			outStr := strings.TrimSpace(string(out))
			if strings.Contains(outStr, "UNIQUE") {
				fmt.Printf("  %s already registered\n", name)
			} else {
				fmt.Printf("  ! Failed to register %s: %s\n", name, outStr)
				failed = append(failed, name)
			}
			continue
		}
		fmt.Printf("  Registered: %s (%s)\n", name, dir)
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to register: %s", strings.Join(failed, ", "))
	}
	return nil
}

// --- Step 5: Daemon & worker hooks ---

func installDaemonAndWorker() error {
	var errs []string

	// Daemon install (launchd plist + config template)
	if runtime.GOOS == "darwin" {
		fmt.Print("  Installing daemon launchd plist...")
		if err := daemon.Install(); err != nil {
			fmt.Printf(" failed: %v\n", err)
			errs = append(errs, "daemon")
		} else {
			fmt.Println(" done")
		}
	} else {
		fmt.Println("  ! Daemon install is macOS-only (launchd)")
	}

	// Worker hook install (taskwarrior on-modify hook)
	fmt.Print("  Installing taskwarrior hooks...")
	if err := worker.Install(); err != nil {
		fmt.Printf(" failed: %v\n", err)
		errs = append(errs, "worker hooks")
	} else {
		fmt.Println(" done")
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to install: %s", strings.Join(errs, ", "))
	}
	return nil
}

// --- Step 6: Verify ---

func verify() error {
	fmt.Println("  Running ttal doctor to verify setup...")
	fmt.Println()

	report := doctor.Run(false)
	doctor.Print(report)

	if report.Errors() > 0 {
		return fmt.Errorf("%d doctor check(s) failed", report.Errors())
	}
	return nil
}

// --- Next steps ---

func printNextSteps() {
	fmt.Println("Next steps:")
	fmt.Println("  1. Create Telegram bots via @BotFather (one per agent)")
	fmt.Println("  2. Add bot tokens to ~/.config/ttal/config.toml")
	fmt.Println("  3. Add your Telegram chat_id to config")
	fmt.Println("  4. Run: ttal doctor  (verify everything is green)")
	fmt.Println("  5. Run: ttal daemon start")
}

// --- Helpers ---

// findAgentDirs returns paths to directories containing a CLAUDE.md file.
func findAgentDirs(workspace string) ([]string, error) {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return nil, fmt.Errorf("cannot read workspace %s: %w", workspace, err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		claudeMd := filepath.Join(workspace, e.Name(), "CLAUDE.md")
		if _, err := os.Stat(claudeMd); err == nil {
			dirs = append(dirs, filepath.Join(workspace, e.Name()))
		}
	}
	return dirs, nil
}

func printAgentDirs(dirs []string) {
	for _, dir := range dirs {
		fmt.Printf("  Agent found: %s\n", filepath.Base(dir))
	}
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
