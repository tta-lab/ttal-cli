package open

import (
	"os"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
)

func TestResolveShellWithConfig_Priority(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		loadErr  error
		envShell string
		want     string
	}{
		{"config shell wins", &config.Config{Shell: "zsh"}, nil, "/usr/bin/fish", "zsh"},
		{"env shell fallback", nil, nil, "/usr/bin/zsh", "/usr/bin/zsh"},
		{"env shell when no config shell", &config.Config{}, nil, "/usr/bin/zsh", "/usr/bin/zsh"},
		{"hardcoded fallback", nil, nil, "", "/bin/sh"},
		{"config error skips to env", nil, nil, "/bin/bash", "/bin/bash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envShell != "" {
				t.Setenv("SHELL", tt.envShell)
			} else {
				_ = os.Unsetenv("SHELL")
			}
			got := resolveShellWithConfig(tt.cfg, tt.loadErr)
			if got != tt.want {
				t.Errorf("resolveShellWithConfig() = %q, want %q", got, tt.want)
			}
		})
	}
}
