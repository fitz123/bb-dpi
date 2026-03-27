#!/bin/bash
# Verify all XRay servers are reachable
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
SERVERS_FILE="$REPO_DIR/servers.json"

[[ -f "$REPO_DIR/.env" ]] && source "$REPO_DIR/.env"

if [[ ! -f "$SERVERS_FILE" ]]; then
    echo "Error: servers.json not found"
    exit 1
fi

fail=0

while IFS= read -r server; do
    name=$(echo "$server" | jq -r '.name')
    host=$(echo "$server" | jq -r '.host')
    ssh_host=$(echo "$server" | jq -r '.ssh')

    echo "=== $name ($host) ==="

    for port in 443 8443; do
        echo -n "  Port $port... "
        if timeout 5 bash -c "echo | nc -z -w5 $host $port" 2>/dev/null; then
            echo "OK"
        else
            echo "FAIL"
            fail=1
        fi
    done

    echo -n "  Container... "
    status=$(ssh -o ConnectTimeout=5 "$ssh_host" "docker inspect --format='{{.State.Health.Status}}' xray 2>/dev/null" || echo "unknown")
    if [[ "$status" == "healthy" ]]; then
        echo "OK ($status)"
    else
        echo "WARN ($status)"
        fail=1
    fi
    echo ""
done < <(jq -c '.[]' "$SERVERS_FILE")

if [[ $fail -eq 0 ]]; then
    echo "All checks passed!"
else
    echo "Some checks failed!"
    exit 1
fi
