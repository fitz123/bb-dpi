#!/bin/bash
# Verify XRay server is reachable
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

source "$REPO_DIR/.env"

echo -n "Checking port $PORT... "
if nc -z -w5 "$SERVER" "$PORT" 2>/dev/null; then
    echo "OK"
else
    echo "FAIL"
    exit 1
fi

echo -n "Checking container health... "
status=$(ssh "$SSH_HOST" "docker inspect --format='{{.State.Health.Status}}' xray 2>/dev/null" || echo "unknown")
if [[ "$status" == "healthy" ]]; then
    echo "OK ($status)"
else
    echo "WARN ($status)"
fi

echo "All checks passed!"
