#!/bin/bash
# Backup XRay config and local secrets from all servers
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
SERVERS_FILE="$REPO_DIR/servers.json"

[[ -f "$REPO_DIR/.env" ]] && source "$REPO_DIR/.env"

BACKUP_DIR="$REPO_DIR/backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup each server's config
if [[ -f "$SERVERS_FILE" ]]; then
    while IFS= read -r server; do
        name=$(echo "$server" | jq -r '.name')
        ssh_host=$(echo "$server" | jq -r '.ssh')
        echo "Backing up $name..."
        scp "$ssh_host:${CONFIG_PATH:-/opt/xray/config.json}" "$BACKUP_DIR/config-${name}.json" || echo "Warning: failed to backup $name"
    done < <(jq -c '.[]' "$SERVERS_FILE")
fi

# Backup local files
cp "$REPO_DIR/.env" "$BACKUP_DIR/" 2>/dev/null || true
cp "$REPO_DIR/users.json" "$BACKUP_DIR/" 2>/dev/null || true
cp "$REPO_DIR/servers.json" "$BACKUP_DIR/" 2>/dev/null || true

echo "Backup saved to: $BACKUP_DIR"
