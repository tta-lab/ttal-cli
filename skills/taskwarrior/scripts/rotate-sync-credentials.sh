#!/bin/bash
# Taskwarrior Sync Credential Rotation Script
#
# Usage: ./rotate-sync-credentials.sh
#
# This script:
# 1. Generates new sync credentials (client ID + encryption secret)
# 2. Updates ~/.taskrc.sync with new credentials
# 3. Displays next steps for encryption via chezmoi

set -euo pipefail

SYNC_CONFIG="$HOME/.taskrc.sync"
SERVER_URL="https://task.flicknote.app"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Taskwarrior Sync Credential Rotation${NC}"
echo "======================================"
echo

# Check if sync config exists
if [ -f "$SYNC_CONFIG" ]; then
    echo -e "${YELLOW}Warning: $SYNC_CONFIG already exists${NC}"
    echo "Current credentials:"
    grep -E "sync\.(server|encryption)" "$SYNC_CONFIG" | sed 's/sync\./  /' || true
    echo
    read -p "Do you want to rotate these credentials? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
    echo
fi

# Generate new credentials
echo -e "${BLUE}Generating new credentials...${NC}"
NEW_CLIENT_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
NEW_SECRET=$(openssl rand -base64 32)

echo -e "${GREEN}✓${NC} New Client ID: $NEW_CLIENT_ID"
echo -e "${GREEN}✓${NC} New Encryption Secret: ${NEW_SECRET:0:10}... (truncated)"
echo

# Create/update sync config
cat > "$SYNC_CONFIG" <<EOF
# TaskChampion sync configuration
# This file is managed by chezmoi and encrypted
# Credentials rotated: $(date +%Y-%m-%d)
sync.server.url=$SERVER_URL
sync.server.client_id=$NEW_CLIENT_ID
sync.encryption_secret=$NEW_SECRET
EOF

echo -e "${GREEN}✓${NC} Updated $SYNC_CONFIG"
echo

# Display next steps
echo -e "${BLUE}Next Steps:${NC}"
echo "1. Encrypt and add to chezmoi:"
echo -e "   ${YELLOW}chezmoi add --encrypt ~/.taskrc.sync${NC}"
echo
echo "2. Verify taskwarrior can read the config:"
echo -e "   ${YELLOW}task show | grep -E 'sync\\.(server|encryption)'${NC}"
echo
echo "3. Initial sync with new credentials:"
echo -e "   ${YELLOW}task sync${NC}"
echo
echo "4. Commit the encrypted file:"
echo -e "   ${YELLOW}cd ~/clawd/dotfiles && git add encrypted_dot_taskrc.sync.age${NC}"
echo -e "   ${YELLOW}git commit -m 'fix(security): rotate taskwarrior sync credentials'${NC}"
echo
echo -e "${BLUE}Note:${NC} Old credentials will be in git history but are now revoked."
echo "      The new client ID creates a fresh copy of your tasks on the server."
