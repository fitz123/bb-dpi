#!/bin/bash
# XRay REALITY Deployment Script
# Deploys vanilla XRay to a fresh server with hardening

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

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

# Load or create .env
load_env() {
    if [[ -f "$REPO_DIR/.env" ]]; then
        source "$REPO_DIR/.env"
        log "Loaded existing .env"
    else
        log "No .env found, using defaults from .env.example"
        source "$REPO_DIR/.env.example"
    fi
}

# Save .env with generated values
save_env() {
    cat > "$REPO_DIR/.env" << EOF
# Server connection
SERVER="$SERVER"
SSH_HOST="$SSH_HOST"

# Container
CONTAINER="$CONTAINER"
CONFIG_PATH="$CONFIG_PATH"
PORT=$PORT

# REALITY parameters
PUBLIC_KEY="$PUBLIC_KEY"
PRIVATE_KEY="$PRIVATE_KEY"
SHORT_ID="$SHORT_ID"
SNI="$SNI"
FINGERPRINT="$FINGERPRINT"
FLOW="$FLOW"
EOF
    success "Saved .env"
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
    # Ensure current user can run docker
    sudo usermod -aG docker "$USER" 2>/dev/null || true
    exit 0
fi

# Install Docker
curl -fsSL https://get.docker.com | sudo sh

# Add current user to docker group
sudo usermod -aG docker "$USER"

# Install docker-compose plugin
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

    # Generate keys using xray on server (use sg docker since group not active yet)
    local keys
    keys=$(ssh "$SSH_HOST" "sg docker -c 'docker run --rm ghcr.io/xtls/xray-core:latest x25519'")

    PRIVATE_KEY=$(echo "$keys" | grep "PrivateKey:" | cut -d' ' -f2)
    PUBLIC_KEY=$(echo "$keys" | grep "Password:" | cut -d' ' -f2)

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

# Generate UUID
generate_uuid() {
    local uuid
    uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    echo "$uuid"
}

# Create server config
create_config() {
    local uuid="$1"
    log "Creating server config..."

    # Read template and substitute
    local config
    config=$(cat "$REPO_DIR/config/server.template.json")
    config=${config//<UUID>/$uuid}
    config=${config//<SNI>/$SNI}
    config=${config//<PRIVATE_KEY>/$PRIVATE_KEY}
    config=${config//<SHORT_ID>/$SHORT_ID}

    # Create config directory and upload
    ssh "$SSH_HOST" "sudo mkdir -p /opt/xray && sudo chown \$USER:\$USER /opt/xray"
    echo "$config" | ssh "$SSH_HOST" "cat > /opt/xray/config.json"

    success "Config created on server"
}

# Upload docker-compose and start
start_container() {
    log "Starting XRay container..."

    # Upload docker-compose.yml
    scp "$REPO_DIR/docker-compose.yml" "$SSH_HOST:/opt/xray/"

    # Start container (use sg docker since group not active yet)
    ssh "$SSH_HOST" "cd /opt/xray && sg docker -c 'docker compose pull && docker compose up -d'"

    # Wait for container to be healthy
    sleep 5

    local status
    status=$(ssh "$SSH_HOST" "sg docker -c 'docker inspect --format={{.State.Health.Status}} xray 2>/dev/null || echo starting'")

    if [[ "$status" == "healthy" || "$status" == "starting" ]]; then
        success "XRay container started"
    else
        warn "Container status: $status"
    fi
}

# Generate VLESS share URL
generate_url() {
    local uuid="$1"
    local name="${2:-Admin}"
    local encoded_name
    encoded_name=$(echo -n "$name" | jq -sRr @uri)

    echo "vless://${uuid}@${SERVER}:${PORT}?encryption=none&flow=${FLOW}&security=reality&sni=${SNI}&fp=${FINGERPRINT}&pbk=${PUBLIC_KEY}&sid=${SHORT_ID}&type=tcp#${encoded_name}"
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
    log "XRay REALITY Deployment"
    echo "═══════════════════════════════════════════════════════"

    cd "$REPO_DIR"
    load_env

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

    # Generate first user UUID
    local admin_uuid
    admin_uuid=$(generate_uuid)
    log "Generated admin UUID: $admin_uuid"

    # Create config
    create_config "$admin_uuid"

    # Start container
    start_container

    # Save .env with generated values
    save_env

    # Initialize users
    init_users "$admin_uuid" "Admin"

    # Output share URL
    echo ""
    echo "═══════════════════════════════════════════════════════"
    success "Deployment complete!"
    echo ""
    echo -e "${GREEN}Share URL for Admin:${NC}"
    generate_url "$admin_uuid" "Admin"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo "  1. Test connection with the URL above"
    echo "  2. Add more users: ./scripts/xray-users add \"Name\""
    echo "  3. Update Clash profile with new server details"
    echo ""
}

main "$@"
