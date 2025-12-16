#!/bin/bash
# Update XRay to latest version
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

source "$REPO_DIR/.env"

echo "Backing up current config..."
"$SCRIPT_DIR/backup.sh"

echo "Pulling latest XRay image and restarting..."
ssh "$SSH_HOST" "cd /opt/xray && docker compose pull && docker compose up -d"

sleep 5

echo "Verifying..."
"$SCRIPT_DIR/verify.sh"

echo "Update complete!"
