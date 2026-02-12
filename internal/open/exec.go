package open

import "os/exec"

// lookPath wraps exec.LookPath for testability.
var lookPath = exec.LookPath
