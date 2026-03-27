#!/bin/bash
# Update XRay to latest version on all servers
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
SERVERS_FILE="$REPO_DIR/servers.json"

[[ -f "$REPO_DIR/.env" ]] && source "$REPO_DIR/.env"

if [[ ! -f "$SERVERS_FILE" ]]; then
    echo "Error: servers.json not found"
    exit 1
fi

echo "Backing up current config..."
"$SCRIPT_DIR/backup.sh"

while IFS= read -r host; do
    echo "Updating $host..."
    ssh "$host" "cd /opt/xray && docker compose pull && docker compose up -d"
    sleep 5
done < <(jq -r '.[].ssh' "$SERVERS_FILE")

echo "Verifying..."
"$SCRIPT_DIR/verify.sh"

echo "Update complete!"
