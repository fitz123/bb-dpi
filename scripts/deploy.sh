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
        '{name:$name, host:$host, ssh:$ssh, public_key:$public_key, private_key:$private_key, short_id:$short_id, xhttp_path:$xhttp_path, xhttp_sni:$xhttp_sni, sni:$sni}
         + (if $relay_upstream == "" then {} else {relay_upstream:$relay_upstream} end)')

    local tmp
    tmp=$(mktemp)
    # Remove existing entry with same name, append new one
    jq --argjson entry "$entry" '[.[] | select(.name != $entry.name)] + [$entry]' "$SERVERS_FILE" > "$tmp"
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

# SSH hardening (if not already done)
if grep -q "^PasswordAuthentication yes" /etc/ssh/sshd_config 2>/dev/null; then
    sudo sed -i 's/^PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
    sudo systemctl restart sshd
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

    # Create config directory and upload
    ssh "$SSH_HOST" "sudo mkdir -p /opt/xray && sudo chown \$USER:\$USER /opt/xray"
    echo "$config" | ssh "$SSH_HOST" "cat > /opt/xray/config.json"

    success "Config created on server ($(jq 'length' "$REPO_DIR/users.json") users)"
}

# Upload docker-compose and start
start_container() {
    log "Starting XRay container..."

    scp "$REPO_DIR/docker-compose.yml" "$SSH_HOST:/opt/xray/"
    ssh "$SSH_HOST" "cd /opt/xray && sg docker -c 'docker compose pull && docker compose up -d'"

    sleep 5

    local status
    status=$(ssh "$SSH_HOST" "sg docker -c 'docker inspect --format={{.State.Health.Status}} xray 2>/dev/null || echo starting'")

    if [[ "$status" == "healthy" || "$status" == "starting" ]]; then
        success "XRay container started"
    else
        warn "Container status: $status"
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
