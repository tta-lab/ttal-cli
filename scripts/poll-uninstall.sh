#!/bin/bash
# poll-uninstall.sh - Remove launchd service for ttal worker poll
#
# Unloads the service and removes the plist file.
#
# Usage: ./scripts/poll-uninstall.sh

set -euo pipefail

PLIST_NAME="io.guion.ttal.poll-completion"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"

# Check if plist exists
if [[ ! -f "$PLIST_PATH" ]]; then
    echo "Service not installed (plist not found at $PLIST_PATH)"
    exit 0
fi

# Remove service using modern bootout API
if launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    echo "Removing service..."
    launchctl bootout "gui/$(id -u)/${PLIST_NAME}" 2>/dev/null || true
fi

# Remove plist
rm -f "$PLIST_PATH"

echo "Service uninstalled."
echo ""
echo "Note: Log files remain at ~/.ttal/poll_completion*.log"
