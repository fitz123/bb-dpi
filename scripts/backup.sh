#!/bin/bash
# Backup XRay config and local secrets
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

source "$REPO_DIR/.env"

BACKUP_DIR="$REPO_DIR/backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup server config
scp "$SSH_HOST:$CONFIG_PATH" "$BACKUP_DIR/config.json"

# Backup local files
cp "$REPO_DIR/.env" "$BACKUP_DIR/" 2>/dev/null || true
cp "$REPO_DIR/users.json" "$BACKUP_DIR/" 2>/dev/null || true

echo "Backup saved to: $BACKUP_DIR"
