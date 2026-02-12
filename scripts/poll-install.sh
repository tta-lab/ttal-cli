#!/bin/bash
# poll-install.sh - Install launchd service for ttal worker poll
#
# Creates and loads a launchd plist that runs `ttal worker poll` every 60 seconds.
#
# Usage: ./scripts/poll-install.sh

set -euo pipefail

PLIST_NAME="io.guion.ttal.poll-completion"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"
TTAL_BIN="$(go env GOPATH)/bin/ttal"
LOG_DIR="$HOME/.ttal"

# Verify ttal is installed
if [[ ! -x "$TTAL_BIN" ]]; then
    echo "Error: ttal not found at $TTAL_BIN"
    echo ""
    echo "Install it first:"
    echo "  cd $(dirname "$0")/.. && make install"
    exit 1
fi

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Unload existing service if present
if launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    echo "Unloading existing service..."
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
fi

# Create plist
cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_NAME}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${TTAL_BIN}</string>
        <string>worker</string>
        <string>poll</string>
    </array>

    <key>StartInterval</key>
    <integer>60</integer>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>${LOG_DIR}/poll_completion_stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/poll_completion_stderr.log</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin:$HOME/.local/bin:$HOME/go/bin</string>
        <key>FORGEJO_URL</key>
        <string>${FORGEJO_URL:-}</string>
        <key>FORGEJO_TOKEN</key>
        <string>${FORGEJO_TOKEN:-}</string>
    </dict>
</dict>
</plist>
EOF

# Restrict permissions — plist may contain FORGEJO_TOKEN
chmod 600 "$PLIST_PATH"

echo "Created plist at: $PLIST_PATH"

# Warn if Forgejo vars are missing
if [[ -z "${FORGEJO_URL:-}" ]]; then
    echo ""
    echo "Warning: FORGEJO_URL is not set. Poll won't be able to check PR status."
    echo "  Set it in your shell config and re-run this script."
fi
if [[ -z "${FORGEJO_TOKEN:-}" ]]; then
    echo ""
    echo "Warning: FORGEJO_TOKEN is not set. Poll won't be able to check PR status."
    echo "  Set it in your shell config and re-run this script."
fi

# Load service
launchctl load "$PLIST_PATH"

echo ""
echo "Service loaded and running."
echo "  Binary: $TTAL_BIN"
echo "  Interval: 60 seconds"
echo ""
echo "Commands:"
echo "  Check status:  launchctl list | grep ttal.poll"
echo "  View logs:     tail -f ~/.ttal/poll_completion.log"
echo "  Uninstall:     ./scripts/poll-uninstall.sh"
