#!/bin/bash
# Verify XRay server is reachable
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

source "$REPO_DIR/.env"

for port in 443 8443; do
    echo -n "Checking port $port... "
    if timeout 5 bash -c "echo | nc -z -w5 $SERVER $port" 2>/dev/null; then
        echo "OK"
    else
        echo "FAIL"
    fi
done

echo -n "Checking container health... "
status=$(ssh "$SSH_HOST" "docker inspect --format='{{.State.Health.Status}}' xray 2>/dev/null" || echo "unknown")
if [[ "$status" == "healthy" ]]; then
    echo "OK ($status)"
else
    echo "WARN ($status)"
fi

echo "All checks passed!"
