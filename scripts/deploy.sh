#!/bin/bash
# XRay REALITY Deployment Script
# Deploys vanilla XRay to a fresh server with hardening
#
# Usage:
#   NAME=aws-st SSH_HOST=xray2 SERVER=1.2.3.4 make deploy   # New server
#   NAME=aws-st make deploy                                   # Redeploy existing
#
# Reads server params from servers.json if NAME exists there.
# Generates new keys/short_id/xhttp_path for new servers.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
SERVERS_FILE="$REPO_DIR/servers.json"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# Load shared .env
load_env() {
    if [[ -f "$REPO_DIR/.env" ]]; then
        source "$REPO_DIR/.env"
        log "Loaded .env"
    elif [[ -f "$REPO_DIR/.env.example" ]]; then
        source "$REPO_DIR/.env.example"
        log "No .env found, using .env.example"
    fi
}

# Initialize servers.json if missing
init_servers_file() {
    if [[ ! -f "$SERVERS_FILE" ]]; then
        echo '[]' > "$SERVERS_FILE"
    fi
}

# Load server params from servers.json by name
load_server() {
    local name="$1"
    local entry
    entry=$(jq -r --arg n "$name" '.[] | select(.name == $n)' "$SERVERS_FILE")
    if [[ -n "$entry" ]]; then
        SSH_HOST=$(echo "$entry" | jq -r '.ssh')
        SERVER=$(echo "$entry" | jq -r '.host')
        PUBLIC_KEY=$(echo "$entry" | jq -r '.public_key')
        PRIVATE_KEY=$(echo "$entry" | jq -r '.private_key')
        SHORT_ID=$(echo "$entry" | jq -r '.short_id')
        XHTTP_PATH=$(echo "$entry" | jq -r '.xhttp_path')
        XHTTP_SNI=$(echo "$entry" | jq -r '.xhttp_sni')
        SNI=$(echo "$entry" | jq -r '.sni')
        RELAY_UPSTREAM=$(echo "$entry" | jq -r '.relay_upstream // ""')
        # Raw dest fields. Empty when absent in servers.json — the new-host
        # path in main() also leaves these empty, then main() computes the
        # effective XHTTP_DEST / SNI_DEST after SNI defaults are applied.
        # save_server() conditionally emits these only when RAW is non-empty,
        # so existing servers without an explicit override stay byte-identical
        # across deploys.
        XHTTP_DEST_RAW=$(echo "$entry" | jq -r '.xhttp_dest // ""')
        SNI_DEST_RAW=$(echo "$entry" | jq -r '.sni_dest // ""')
        return 0
    fi
    return 1
}

# Load upstream server's connection params (for relay mode).
# Reads from servers.json. Exports UPSTREAM_* vars. Fails if name not found.
load_upstream() {
    local upstream_name="$1"
    local entry
    entry=$(jq -r --arg n "$upstream_name" '.[] | select(.name == $n)' "$SERVERS_FILE")
    if [[ -z "$entry" ]]; then
        error "relay_upstream '$upstream_name' not found in servers.json"
    fi
    UPSTREAM_HOST=$(echo "$entry" | jq -r '.host')
    UPSTREAM_PUBLIC_KEY=$(echo "$entry" | jq -r '.public_key')
    UPSTREAM_SHORT_ID=$(echo "$entry" | jq -r '.short_id')
    UPSTREAM_XHTTP_PATH=$(echo "$entry" | jq -r '.xhttp_path')
    UPSTREAM_XHTTP_SNI=$(echo "$entry" | jq -r '.xhttp_sni')
    UPSTREAM_SNI=$(echo "$entry" | jq -r '.sni')
    success "Loaded upstream '$upstream_name' ($UPSTREAM_HOST)"
}

# Resolve the relay user UUID from users.json. Relay user name == relay server name
# (e.g., server "gigi" uses a user also named "gigi" to auth its upstream chain).
# Exports RELAY_USER_UUID. Fails if user doesn't exist.
load_relay_user_uuid() {
    local user_name="$1"
    local matches
    matches=$(jq -r --arg n "$user_name" '[to_entries[] | select(.value==$n)] | length' "$REPO_DIR/users.json")
    if [[ "$matches" -eq 0 ]]; then
        error "relay user '$user_name' not found in users.json. Run: ./scripts/xray-users add $user_name"
    fi
    if [[ "$matches" -gt 1 ]]; then
        error "users.json has $matches users named '$user_name' — names must be unique for relay use"
    fi
    RELAY_USER_UUID=$(jq -r --arg n "$user_name" 'first(to_entries[] | select(.value==$n) | .key)' "$REPO_DIR/users.json")
    success "Loaded relay user '$user_name' UUID ($(echo "$RELAY_USER_UUID" | head -c 8)...)"
}

# Save/update server entry in servers.json
save_server() {
    local name="$1"
    local entry
    entry=$(jq -n \
        --arg name "$name" \
        --arg host "$SERVER" \
        --arg ssh "$SSH_HOST" \
        --arg public_key "$PUBLIC_KEY" \
        --arg private_key "$PRIVATE_KEY" \
        --arg short_id "$SHORT_ID" \
        --arg xhttp_path "$XHTTP_PATH" \
        --arg xhttp_sni "${XHTTP_SNI:-dl.google.com}" \
        --arg sni "${SNI:-dl.google.com}" \
        --arg relay_upstream "${RELAY_UPSTREAM:-}" \
        --arg xhttp_dest "${XHTTP_DEST_RAW:-}" \
        --arg sni_dest "${SNI_DEST_RAW:-}" \
        '{name:$name, host:$host, ssh:$ssh, public_key:$public_key, private_key:$private_key, short_id:$short_id, xhttp_path:$xhttp_path, xhttp_sni:$xhttp_sni, sni:$sni}
         + (if $relay_upstream == "" then {} else {relay_upstream:$relay_upstream} end)
         + (if $xhttp_dest == "" then {} else {xhttp_dest:$xhttp_dest} end)
         + (if $sni_dest == "" then {} else {sni_dest:$sni_dest} end)')

    local tmp
    tmp=$(mktemp)
    # Replace existing entry by name, merging the deploy-derived fields ($entry)
    # over the prior entry. This preserves any operator-set fields that
    # deploy.sh doesn't own (e.g. client_render, relay_sources, future per-server
    # knobs) — they all live through redeploys without explicit support here.
    #
    # $entry's keys override matching keys in $prior. $entry is built from the
    # deploy params, so it carries the authoritative current host/keys/SNI.
    jq --argjson entry "$entry" '
        ([.[] | select(.name == $entry.name)] | .[0] // {}) as $prior
        | [.[] | select(.name != $entry.name)] + [$prior + $entry]
    ' "$SERVERS_FILE" > "$tmp"
    mv "$tmp" "$SERVERS_FILE"
    success "Saved server '$name' to servers.json"
}

# Harden server
harden_server() {
    log "Hardening server..."

    ssh "$SSH_HOST" bash << 'REMOTE'
set -e

# Update system
sudo apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -qq

# Install essentials
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    ufw unattended-upgrades curl netcat-openbsd

# Configure UFW
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 443/tcp
sudo ufw allow 8443/tcp
sudo ufw allow 80/tcp
echo "y" | sudo ufw enable || true

# SSH hardening: disable all password-class auth via a high-priority drop-in.
# Why a drop-in (not main-file sed): Ubuntu's main /etc/ssh/sshd_config has
# `Include /etc/ssh/sshd_config.d/*.conf` near the top, so .d files load
# FIRST. For sshd, the FIRST occurrence of a directive wins. Cloud images
# (Selectel, AWS, etc.) often ship /etc/ssh/sshd_config.d/50-cloud-init.conf
# with `PasswordAuthentication yes` — which silently overrides whatever the
# main file says. A `00-`-prefixed drop-in loads before any provider one and
# wins for first-occurrence directives. Idempotent: re-running just rewrites.
sudo tee /etc/ssh/sshd_config.d/00-bb-dpi-hardening.conf > /dev/null <<'SSH_HARDEN'
# Managed by bb-dpi/scripts/deploy.sh — regenerated on every deploy.
PasswordAuthentication no
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
SSH_HARDEN
# Validate FIRST — exit before any reload if syntax is bad (avoids reloading
# sshd with a broken config, which is the lockout scenario this path exists
# to prevent). Operator precedence note: do NOT chain `sshd -t && reload ssh
# || reload sshd` — `&&`/`||` are left-associative same-precedence in bash,
# so a failed `sshd -t` still triggers the `|| reload sshd` branch.
if ! sudo sshd -t; then
    echo "ERROR: sshd config validation failed — refusing to reload" >&2
    exit 1
fi
# Reload (preserves open sessions). Service name is `ssh` on Debian/Ubuntu,
# `sshd` on some RHEL-family images — try both.
sudo systemctl reload ssh 2>/dev/null || sudo systemctl reload sshd

# Verify effective state — bail loudly if any password-class auth is still on.
EFFECTIVE=$(sudo sshd -T 2>/dev/null | awk '/^(passwordauthentication|kbdinteractiveauthentication|challengeresponseauthentication) / {print $1"="$2}' | sort | tr '\n' ' ')
if [[ "$EFFECTIVE" != *"passwordauthentication=no"* ]] || [[ "$EFFECTIVE" != *"kbdinteractiveauthentication=no"* ]]; then
    echo "ERROR: password-class auth not fully disabled after hardening drop-in. Effective: $EFFECTIVE" >&2
    exit 1
fi

# Enable unattended upgrades
echo 'APT::Periodic::Update-Package-Lists "1";' | sudo tee /etc/apt/apt.conf.d/20auto-upgrades > /dev/null
echo 'APT::Periodic::Unattended-Upgrade "1";' | sudo tee -a /etc/apt/apt.conf.d/20auto-upgrades > /dev/null

echo "Server hardening complete"
REMOTE

    success "Server hardened"
}

# Install Docker
install_docker() {
    log "Installing Docker..."

    ssh "$SSH_HOST" bash << 'REMOTE'
set -e

if command -v docker &> /dev/null; then
    echo "Docker already installed"
    sudo usermod -aG docker "$USER" 2>/dev/null || true
    exit 0
fi

curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
sudo apt-get install -y -qq docker-compose-plugin
sudo systemctl enable docker
sudo systemctl start docker

echo "Docker installed"
REMOTE

    success "Docker installed"
}

# Generate REALITY keys on server
generate_keys() {
    log "Generating REALITY keys..."

    local keys
    keys=$(ssh "$SSH_HOST" "sg docker -c 'docker run --rm ghcr.io/xtls/xray-core:latest x25519'")

    # Handle multiple xray output formats:
    #   old:    "Private key: <val>"  / "Public key: <val>"
    #   newer:  "PrivateKey: <val>"   / "PublicKey: <val>"
    #   newest: "PrivateKey: <val>"   / "Password (PublicKey): <val>"
    PRIVATE_KEY=$(echo "$keys" | grep -E "Private" | awk '{print $NF}')
    PUBLIC_KEY=$(echo "$keys" | grep -E "PublicKey|Public key" | awk '{print $NF}')

    if [[ -z "$PRIVATE_KEY" || -z "$PUBLIC_KEY" ]]; then
        error "Failed to generate keys"
    fi

    success "Generated REALITY keys"
    log "Public key: $PUBLIC_KEY"
}

# Generate SHORT_ID
generate_short_id() {
    SHORT_ID=$(openssl rand -hex 8)
    success "Generated short ID: $SHORT_ID"
}

# Generate random XHTTP path
generate_xhttp_path() {
    XHTTP_PATH=$(openssl rand -hex 8)
    success "Generated XHTTP path: $XHTTP_PATH"
}

# Create server config with all users from users.json.
# If RELAY_UPSTREAM is set: uses server-relay.template.json and adds upstream-chain outbounds.
# Otherwise: uses server.template.json (standard exit-server mode).
#
# Args:
#   $1 - server name (used as the relay user name in relay mode; required)
create_config() {
    local name="$1"
    if [[ -z "$name" ]]; then
        error "create_config: name argument is required"
    fi
    log "Creating server config..."

    local template config
    if [[ -n "${RELAY_UPSTREAM:-}" ]]; then
        log "Relay mode: chaining to upstream '$RELAY_UPSTREAM'"
        load_upstream "$RELAY_UPSTREAM"
        # The relay user is named after the relay server itself (e.g., server "gigi" uses user "gigi")
        load_relay_user_uuid "$name"
        template="$REPO_DIR/config/server-relay.template.json"
    else
        template="$REPO_DIR/config/server.template.json"
    fi

    config=$(cat "$template")
    config=${config//<SNI>/$SNI}
    config=${config//<PRIVATE_KEY>/$PRIVATE_KEY}
    config=${config//<SHORT_ID>/$SHORT_ID}
    config=${config//<XHTTP_PATH>/$XHTTP_PATH}
    config=${config//<XHTTP_SNI>/$XHTTP_SNI}
    config=${config//<XHTTP_DEST>/$XHTTP_DEST}
    config=${config//<SNI_DEST>/$SNI_DEST}

    # Relay-mode upstream-chain placeholders (no-op if not in relay mode)
    if [[ -n "${RELAY_UPSTREAM:-}" ]]; then
        config=${config//<UPSTREAM_HOST>/$UPSTREAM_HOST}
        config=${config//<UPSTREAM_PUBLIC_KEY>/$UPSTREAM_PUBLIC_KEY}
        config=${config//<UPSTREAM_SHORT_ID>/$UPSTREAM_SHORT_ID}
        config=${config//<UPSTREAM_XHTTP_PATH>/$UPSTREAM_XHTTP_PATH}
        config=${config//<UPSTREAM_XHTTP_SNI>/$UPSTREAM_XHTTP_SNI}
        config=${config//<UPSTREAM_SNI>/$UPSTREAM_SNI}
        config=${config//<RELAY_USER_UUID>/$RELAY_USER_UUID}
    fi

    # Build clients arrays from users.json (all users)
    local xhttp_clients tcp_clients
    xhttp_clients=$(jq -c '[keys[] | {id: .}]' "$REPO_DIR/users.json")
    tcp_clients=$(jq -c --arg flow "${FLOW:-xtls-rprx-vision}" '[keys[] | {id: ., flow: $flow}]' "$REPO_DIR/users.json")

    # Replace template placeholder with full client arrays
    config=$(echo "$config" | jq \
        --argjson xhttp "$xhttp_clients" \
        --argjson tcp "$tcp_clients" \
        '.inbounds |= map(
            if .streamSettings.network == "xhttp" then .settings.clients = $xhttp
            else .settings.clients = $tcp end)')

    # Create config directory and upload to STAGING path. start_container()
    # then pulls the image, validates this staged file with `xray -test`
    # inside `docker run --rm` (so the image-correct binary is used), and
    # only atomically mv's it into place if validation passes. Failure leaves
    # the running container untouched on the previous valid config.
    ssh -o ConnectTimeout=10 "$SSH_HOST" "sudo mkdir -p /opt/xray && sudo chown \$USER:\$USER /opt/xray"
    # Write via .partial + rename so a dropped SSH session never leaves a
    # truncated staging file. start_container()'s xray -test runs against the
    # complete file or against nothing — never against a half-written JSON
    # that would surface as a confusing renderer-shaped error.
    echo "$config" | ssh -o ConnectTimeout=10 "$SSH_HOST" "cat > /opt/xray/config.staging.json.partial && mv /opt/xray/config.staging.json.partial /opt/xray/config.staging.json"

    success "Config staged at /opt/xray/config.staging.json ($(jq 'length' "$REPO_DIR/users.json") users)"
}

# Upload docker-compose, validate staged config against the pulled image,
# atomically swap to live path, then start/restart container.
#
# Order is critical: pull image BEFORE validating, so the binary that will
# run is the one that approved the config. Validating with `docker exec`
# against a still-running old container can pass a config that the newly
# pulled image then rejects on restart — leaving :443 down.
start_container() {
    log "Starting XRay container..."

    scp "$REPO_DIR/docker-compose.yml" "$SSH_HOST:/opt/xray/"

    log "Pulling image (so validation uses the binary that will run)..."
    # `timeout` caps an image pull that gets wedged on a slow registry. SSH
    # ConnectTimeout catches an unreachable host fast. Per AGENTS.md scripting
    # guidelines: every network/external call gets a timeout.
    ssh -o ConnectTimeout=10 "$SSH_HOST" "cd /opt/xray && timeout 300 sg docker -c 'docker compose pull'"

    # Extract the xray service's image from docker-compose.yml. Track the
    # `xray:` line's indent and treat ONLY same-or-lesser-indent keys as the
    # next service (so nested keys at deeper indent like `volumes:`/
    # `logging:`/`healthcheck:` don't terminate the scan early — this matters
    # because the previous awk only worked when `image:` was the first key
    # under xray). Strip optional quotes and inline comments. Portable awk
    # (BSD + GNU): uses match()/RLENGTH for indent measurement.
    local image validate_cmd
    image=$(awk '
        /^[[:space:]]*xray:[[:space:]]*$/ {
            match($0, /^[[:space:]]*/); xray_indent=RLENGTH
            in_xray=1; next
        }
        in_xray {
            match($0, /^[[:space:]]*/); cur_indent=RLENGTH
            # End of xray block: another key at the same indent as xray:
            # (sibling service) or shallower (top-level key).
            if (cur_indent <= xray_indent && /^[[:space:]]*[a-zA-Z][a-zA-Z0-9_-]*:/) {
                in_xray=0
            }
        }
        in_xray && /^[[:space:]]*image:[[:space:]]+/ {
            sub(/^[[:space:]]*image:[[:space:]]+/, "")
            sub(/[[:space:]]*#.*$/, "")
            gsub(/^["'\'']|["'\'']$/, "")
            print; exit
        }
    ' "$REPO_DIR/docker-compose.yml")
    [[ -z "$image" ]] && error "Could not extract xray image from docker-compose.yml"

    log "Validating /opt/xray/config.staging.json against image $image..."
    # Use `sg docker -c` (matching existing pattern); `docker run --rm` against
    # the explicit image — NOT `docker exec` (which would use the still-running
    # old container's image). Mount /opt/xray read-only at /etc/xray inside.
    # `timeout 60` caps a docker run that wedges on image pull race / network /
    # daemon hang; ssh ConnectTimeout caps the round-trip itself. Per AGENTS.md.
    validate_cmd="timeout 60 sg docker -c 'docker run --rm -v /opt/xray:/etc/xray:ro $image -test -config /etc/xray/config.staging.json'"
    if ! ssh -o ConnectTimeout=10 "$SSH_HOST" "$validate_cmd"; then
        error "xray -test rejected config. Old config still live. New config left at /opt/xray/config.staging.json for inspection."
    fi
    success "xray -test passed"

    # Snapshot the current live config so we can roll back if the restart
    # fails. `cp` (not mv) — we still need config.json live during the up/restart
    # so a transient failure-then-success path doesn't briefly serve no config.
    # Missing config.json is OK (first deploy); cp failure (perms, disk full,
    # SSH drop) is NOT OK — without a snapshot there's no rollback path, so
    # abort before swapping in the new config. Test the file's existence
    # remotely inside the ssh command so `set -e` here doesn't bail on a
    # legitimate first-deploy absence.
    if ! ssh -o ConnectTimeout=10 "$SSH_HOST" 'if [ -f /opt/xray/config.json ]; then cp /opt/xray/config.json /opt/xray/config.json.prev; fi'; then
        error "Failed to snapshot live config.json to .prev — aborting before swap so a rollback remains possible."
    fi

    # Atomic swap (rename within same fs). /opt/xray is chowned to the deploy
    # user in create_config(), so no sudo needed.
    ssh -o ConnectTimeout=10 "$SSH_HOST" "mv /opt/xray/config.staging.json /opt/xray/config.json"

    # `docker compose up -d` is idempotent: when neither image nor compose file
    # changed, it leaves the running container alone. But config.json IS
    # bind-mounted and we just rewrote it — xray-core only reads it at process
    # start, so we MUST explicitly restart to apply config-only edits. Without
    # this, redeploys can silently leave xray running with the previous config
    # (observed: SNI swap stuck on the prior dest's cert until manual restart).
    local rollback_needed=0
    if ! ssh -o ConnectTimeout=10 "$SSH_HOST" "cd /opt/xray && timeout 120 sg docker -c 'docker compose up -d'"; then
        warn "compose up -d failed"
        rollback_needed=1
    fi

    if [[ $rollback_needed -eq 0 ]]; then
        log "Restarting xray to reload config.json..."
        if ! ssh -o ConnectTimeout=10 "$SSH_HOST" "cd /opt/xray && timeout 60 sg docker -c 'docker compose restart xray'"; then
            warn "compose restart failed"
            rollback_needed=1
        fi
    fi

    if [[ $rollback_needed -eq 0 ]]; then
        # Poll for `state=running, health=healthy` up to 45s. docker-compose.yml
        # has start_period=10s + interval=30s — so the first healthcheck can't
        # even fire before t=10s, and a slow boot may stay `starting` until
        # ~40s. A bare 5s sleep + single inspect ALWAYS lands in `starting`,
        # silently green-lighting a deploy whose healthcheck would later fail.
        # The loop exits early on hard failure (state ∉ {running, "", "starting"})
        # so we don't wait 45s on a crashed container. `timeout 5` around the
        # remote docker inspect caps a wedged docker daemon — AGENTS.md.
        # Output is `<state>|<health>`; `|` not used in docker status strings.
        local deadline=$((SECONDS + 45))
        local inspect_out state="" health=""
        while [ $SECONDS -lt $deadline ]; do
            inspect_out=$(ssh -o ConnectTimeout=10 "$SSH_HOST" "timeout 5 sg docker -c 'docker inspect --format={{.State.Status}}|{{.State.Health.Status}} xray'" 2>/dev/null || echo "missing|missing")
            state="${inspect_out%|*}"
            health="${inspect_out#*|}"

            if [[ "$state" == "running" && "$health" == "healthy" ]]; then
                break
            fi
            if [[ "$state" != "running" && "$state" != "" ]]; then
                # Hard fail — container is exited/dead/missing. No point waiting.
                break
            fi
            sleep 5
        done

        case "$state" in
            running)
                case "$health" in
                    healthy) success "XRay container running (health=healthy)" ;;
                    starting) warn "Container running but still 'starting' after 45s — healthcheck not green within deadline"; rollback_needed=1 ;;
                    *)        warn "Container running but health=$health (expected healthy)"; rollback_needed=1 ;;
                esac
                ;;
            *)
                warn "Container state=$state (expected running)"
                rollback_needed=1
                ;;
        esac
    fi

    if [[ $rollback_needed -eq 1 ]]; then
        warn "Rolling back to previous config.json..."
        # Restore config.json from snapshot. Missing snapshot = first-deploy
        # half-state; surface that explicitly instead of silently swallowing.
        # A failed mv on an existing snapshot means rollback didn't actually
        # happen — fail loudly so the operator doesn't think they're recovered.
        #
        # Capture pattern: explicit `if !` wraps `$()` so a non-zero ssh exit
        # is handled here instead of triggering `set -e` and skipping the case.
        local restore_status
        if ! restore_status=$(ssh -o ConnectTimeout=10 "$SSH_HOST" 'if [ -f /opt/xray/config.json.prev ]; then mv /opt/xray/config.json.prev /opt/xray/config.json && echo restored; else echo no_prev; fi'); then
            error "Deploy failed AND rollback SSH/probe failed. Inspect /opt/xray on the remote manually — config.json may be the just-rejected one."
        fi
        case "$restore_status" in
            restored)
                if ! ssh -o ConnectTimeout=10 "$SSH_HOST" "cd /opt/xray && timeout 60 sg docker -c 'docker compose restart xray'"; then
                    error "Deploy failed AND rollback restart failed — xray may be down. SSH and inspect manually."
                fi
                error "Deploy failed; restored previous config.json and restarted."
                ;;
            no_prev)
                error "Deploy failed on a first-deploy; no previous config.json to restore. Half-state on remote: new config.json on disk, no running container. Inspect /opt/xray and container logs."
                ;;
            *)
                error "Deploy failed AND rollback snapshot probe returned unexpected status='$restore_status'. Inspect /opt/xray manually."
                ;;
        esac
    fi

    # Successful rollout — drop the previous-config snapshot. Best-effort:
    # a transient SSH failure here must NOT abort start_container, because
    # main() runs save_server() afterwards and a first-deploy that fails
    # this cleanup but succeeded everything else would leave the operator
    # with deployed secrets that were never persisted to servers.json.
    # A leftover config.json.prev is harmless (overwritten on next deploy).
    if ! ssh -o ConnectTimeout=10 "$SSH_HOST" "rm -f /opt/xray/config.json.prev"; then
        warn "Could not remove /opt/xray/config.json.prev (SSH transient?). Non-fatal — will be overwritten on the next deploy."
    fi
}

# Initialize users.json with first user
init_users() {
    local uuid="$1"
    local name="$2"

    cat > "$REPO_DIR/users.json" << EOF
{
  "$uuid": "$name"
}
EOF
    success "Initialized users.json"
}

# Main deployment
main() {
    local name="${NAME:-}"

    if [[ -z "$name" ]]; then
        error "NAME is required. Usage: NAME=my-server SSH_HOST=host SERVER=ip make deploy"
    fi

    log "XRay REALITY Deployment: $name"
    echo "═══════════════════════════════════════════════════════"

    cd "$REPO_DIR"
    load_env
    init_servers_file

    # Load existing server or require SSH_HOST+SERVER for new
    if load_server "$name"; then
        log "Found existing server '$name' in servers.json (redeploy)"
    else
        if [[ -z "${SSH_HOST:-}" || -z "${SERVER:-}" ]]; then
            error "New server: SSH_HOST and SERVER are required"
        fi
        log "New server: $name ($SERVER via $SSH_HOST)"
    fi

    # Set defaults
    SNI="${SNI:-dl.google.com}"
    XHTTP_SNI="${XHTTP_SNI:-dl.google.com}"

    # Raw dest overrides: empty when servers.json doesn't carry the field
    # (existing behavior — REALITY fallback dest = matching SNI on :443).
    # Set non-empty if you want REALITY to fall back to a local service
    # (e.g., xhttp_dest=127.0.0.1:8081 routes browsers and probes to a
    # co-hosted local nginx serving an LE-certed public hostname).
    : "${XHTTP_DEST_RAW:=}"
    : "${SNI_DEST_RAW:=}"
    XHTTP_DEST="${XHTTP_DEST_RAW:-${XHTTP_SNI}:443}"
    SNI_DEST="${SNI_DEST_RAW:-${SNI}:443}"

    # Defensive shape check on dest values reaching the template. Cheap, catches
    # typos like `127.0.0.1::8081` or `127.0.0.1 8081` BEFORE we pay for an SSH
    # round-trip + `xray -test` on the remote. servers.json is operator-owned so
    # this isn't a security boundary; it's a faster failure.
    if [[ ! "$XHTTP_DEST" =~ ^[a-zA-Z0-9._-]+:[0-9]+$ ]]; then
        error "XHTTP_DEST='$XHTTP_DEST' does not match host:port"
    fi
    if [[ ! "$SNI_DEST" =~ ^[a-zA-Z0-9._-]+:[0-9]+$ ]]; then
        error "SNI_DEST='$SNI_DEST' does not match host:port"
    fi

    # Check SSH connectivity
    log "Testing SSH connection to $SSH_HOST..."
    ssh -o ConnectTimeout=10 "$SSH_HOST" "echo 'SSH OK'" || error "Cannot connect to $SSH_HOST"
    success "SSH connection OK"

    # Harden server
    harden_server

    # Install Docker
    install_docker

    # Generate secrets if not set
    if [[ -z "${PRIVATE_KEY:-}" || -z "${PUBLIC_KEY:-}" ]]; then
        generate_keys
    fi

    if [[ -z "${SHORT_ID:-}" ]]; then
        generate_short_id
    fi

    if [[ -z "${XHTTP_PATH:-}" ]]; then
        generate_xhttp_path
    fi

    # Use existing users or create first user
    if [[ -f "$REPO_DIR/users.json" ]] && [[ $(jq 'length' "$REPO_DIR/users.json") -gt 0 ]]; then
        log "Using existing users.json ($(jq 'length' "$REPO_DIR/users.json") users)"
    else
        local admin_uuid
        admin_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
        log "Generated admin UUID: $admin_uuid"
        init_users "$admin_uuid" "Admin"
    fi

    # Create config with all users
    create_config "$name"

    # Start container
    start_container

    # Save server to servers.json
    save_server "$name"

    echo ""
    echo "═══════════════════════════════════════════════════════"
    success "Deployment complete: $name ($SERVER)"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo "  1. Render client configs: ./scripts/render-config"
    echo "  2. Add more users: ./scripts/xray-users add \"Name\""
    echo "  3. Get share URLs: ./scripts/xray-users url \"Name\""
    echo ""
}

main "$@"
