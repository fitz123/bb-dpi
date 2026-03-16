# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

XRay REALITY VPN with sing-box client infrastructure. Two components:
- **Server**: XRay VLESS REALITY on Docker (deploy, update, user management)
- **Client**: sing-box with embedded Tailscale on macOS (config generation, install, start/stop)

Pure Bash scripts with no build process.

## Commands

### Server Management
```bash
make deploy              # First-time server deployment (SSH hardening, Docker, XRay)
make update              # Update XRay to latest version
make verify              # Health check (port 443 + container status)
make backup              # Backup config and secrets
make list                # List users

# User management
./scripts/xray-users add "Device Name"
./scripts/xray-users url "Device Name"
./scripts/xray-users remove "Device Name"
./scripts/xray-users sync    # Push local users.json to server
```

### Client Management
```bash
# Generate client package (config + scripts + ZIP)
./scripts/generate-client-config "Device Name" [tailscale-auth-key]
./scripts/generate-client-config "vless://uuid@host:port?..."

# Render config from template using .env values
./scripts/render-config [template] [output]

# Validate sing-box config
./scripts/validate-config [config.json]

# Start/stop VPN (installed to ~/.local/bin/)
vpn-start [--render]     # Start sing-box with search domain config
vpn-stop                 # Stop sing-box and cleanup

# Install client package on target Mac
./scripts/vpn-install [package-dir]
```

## Architecture

### Server Side
- XRay config.json with UUIDs in `/xray/config/config.json` on remote server
- Template in `config/server.template.json` gets variable substitution at deploy time
- Docker container with security hardening (cap_drop: ALL, read_only, no-new-privileges)

### Client Side (sing-box + embedded Tailscale)
- sing-box 1.13+ with embedded Tailscale endpoint (no standalone Tailscale.app needed)
- Template in `config/client/sing-box.template.json` rendered via `envsubst`
- Config installed to `~/.config/sing-box/config.json`
- Custom binary can be placed in `local/bin/sing-box` (takes priority over brew)

### Traffic Routing
- Tailscale peers (100.x.x.x) → Tailscale endpoint
- Corporate subnets (10.0.0.0/8, 172.16.0.0/12) → Tailscale endpoint
- Russian domains/IPs (.ru, geoip-ru) → Direct (bypass VPN)
- Local network (192.168.x.x) → Direct
- Everything else → XRay VLESS proxy

### Local State
- `users.json` maps UUIDs to friendly device names
- `.env` stores connection params (SERVER, PUBLIC_KEY, SHORT_ID, SNI, etc.)

### SSH-Based Operations
All server management happens via SSH. **Always use `ssh-xray` as the SSH host** for connecting to the XRay server. Scripts use `ssh $SSH_HOST` for commands and `scp` for file transfers.

### Key Scripts
- `scripts/deploy.sh` - Server hardening (UFW, SSH keys-only, unattended-upgrades), Docker install, REALITY key generation, container startup
- `scripts/xray-users` - User CRUD with UUID generation, config.json manipulation via `jq`, VLESS URL generation
- `scripts/generate-client-config` - Render client config, package with scripts, create ZIP
- `scripts/render-config` - Template renderer using `envsubst`
- `scripts/validate-config` - Validate config via `sing-box check`
- `scripts/vpn-install` - Install sing-box (via brew), config, scripts, Finder shortcuts
- `scripts/vpn-start` - Start sing-box with search domain and logging
- `scripts/vpn-stop` - Stop sing-box and remove search domain

### Security Layers
1. UFW firewall (ports 22, 443 only)
2. SSH key-only authentication
3. Docker: `cap_drop: ALL`, `read_only: true`, `no-new-privileges: true`
4. REALITY protocol encryption

## Required Local Tools

`bash`, `ssh`, `scp`, `jq`, `uuidgen`, `openssl`, `nc`, `envsubst`, `sing-box` (via brew or custom build)

## Scripting Guidelines

- **Always use timeouts** - Every network operation, external command, or potentially blocking call must have a timeout to prevent scripts from hanging. Use `timeout <seconds> <command>` for commands, or built-in timeouts for tools like `curl -m`, `nc -w`, etc.

## Config Change Safety

- **NEVER restart apps/services before validating config** - Always test configuration syntax/validity BEFORE restarting. A bad config can break VPN connectivity and lock you out.
- For sing-box: use `sing-box check -c config.json` or `./scripts/validate-config` before restart
- For XRay: use `xray -test -config config.json` before container restart
- General rule: read the generated/merged config file and verify your changes are present and correct

## Files That Should Never Be Committed

`.env`, `users.json` - contain secrets and user UUIDs
