package sandbox

import (
	"cmp"
	"runtime"
	"time"
)

// Options configures the sandbox constructor.
type Options struct {
	BwrapPath        string // Linux only; defaults to "bwrap"
	Timeout          time.Duration
	AllowUnsandboxed bool // if true, fall back to NoopSandbox when no platform sandbox is found
}

// New creates the appropriate sandbox for the current platform.
// On macOS, it returns a SeatbeltSandbox if sandbox-exec is available.
// On Linux, it returns a BwrapSandbox if bwrap is available.
// Falls back to NoopSandbox when AllowUnsandboxed is true and no platform sandbox is found.
// Returns UnavailableSandbox when AllowUnsandboxed is false and no platform sandbox is found.
func New(opts Options) Sandbox {
	switch runtime.GOOS {
	case "darwin":
		sbx := &SeatbeltSandbox{Timeout: opts.Timeout}
		if sbx.IsAvailable() {
			return sbx
		}
	case "linux":
		sbx := &BwrapSandbox{BwrapPath: cmp.Or(opts.BwrapPath, "bwrap"), Timeout: opts.Timeout}
		if sbx.IsAvailable() {
			return sbx
		}
	}
	if opts.AllowUnsandboxed {
		return &NoopSandbox{Timeout: opts.Timeout}
	}
	return &UnavailableSandbox{Platform: runtime.GOOS}
}
