package sync

import (
	"fmt"
	"os"
	"path/filepath"
)

// configFiles is the allowlist of TOML files deployed from team_path to ~/.config/ttal/.
// config.toml is excluded — it contains machine-specific settings (chat_id, paths).
// .env and license are excluded — secrets and license keys.
var configFiles = []string{
	"prompts.toml",
	"roles.toml",
	"pipelines.toml",
	"sandbox.toml",
}

// ConfigResult tracks a single config file deployment for reporting.
type ConfigResult struct {
	Source string
	Name   string
	Dest   string
}

// DeployConfigs copies allowlisted TOML files from teamPath to configDir.
// Only files that exist in teamPath are deployed. Missing files are silently skipped.
func DeployConfigs(teamPath, configDir string, dryRun bool) ([]ConfigResult, error) {
	if !dryRun {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating config dir %s: %w", configDir, err)
		}
	}

	var results []ConfigResult
	for _, name := range configFiles {
		src := filepath.Join(teamPath, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(configDir, name)
		if !dryRun {
			if err := copyFile(src, dst); err != nil {
				return nil, fmt.Errorf("deploying %s: %w", name, err)
			}
		}
		results = append(results, ConfigResult{
			Source: src,
			Name:   name,
			Dest:   dst,
		})
	}
	return results, nil
}
