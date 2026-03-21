# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

XRay REALITY VPN with auto-failover client infrastructure. Two components:
- **Server**: XRay VLESS REALITY on Docker — XHTTP (port 443, primary) + TCP+vision (port 8443, fallback)
- **Client**: sing-box TUN with urltest auto-failover between xray-core SOCKS (XHTTP) and sing-box native (TCP+vision), plus embedded Tailscale for corporate access

Pure Bash scripts with no build process.

## Commands

### Server Management
```bash
make deploy              # First-time server deployment (SSH hardening, Docker, XRay)
make update              # Update XRay to latest version
make verify              # Health check (ports 443, 8443 + container status)
make backup              # Backup config and secrets
make list                # List users

# User management
./scripts/xray-users add "Device Name"
./scripts/xray-users url "Device Name"    # Outputs XHTTP + TCP+vision URLs
./scripts/xray-users remove "Device Name"
./scripts/xray-users sync    # Push local users.json to server
```

### Client Management
```bash
# Render both configs (sing-box-auto + xray-xhttp) from templates
./scripts/render-config

# Validate sing-box config
./scripts/validate-config [config.json]

# Start/stop VPN (launchd services, installed to ~/.local/bin/)
vpn-start              # Start xray-core + sing-box via launchd
vpn-start --render     # Render from templates first, then start
vpn-stop               # Unload launchd services and cleanup

# Generate client package (config + scripts + ZIP)
./scripts/generate-client-config "Device Name" [tailscale-auth-key]
./scripts/generate-client-config "vless://uuid@host:port?..."

# Install client package on target Mac
./scripts/vpn-install [package-dir]
```

## Architecture

### Server Side
- XRay with two inbounds: XHTTP on 443, TCP+vision on 8443
- Template in `config/server.template.json` gets variable substitution at deploy time
- XHTTP settings: `mode: auto`, `xPaddingBytes` for DPI resistance
- Docker container with `network_mode: host`, `read_only: true`, `no-new-privileges`

### Client Side (sing-box + xray-core + embedded Tailscale)
- sing-box 1.13+ with embedded Tailscale endpoint (no standalone Tailscale.app needed)
- urltest outbound probes both transports every 30s with `interrupt_exist_connections`
- xray-core runs as launchd service (`com.xray-xhttp`), provides SOCKS proxy on 127.0.0.1:1080 for XHTTP
- sing-box runs as launchd service (`com.sing-box-vpn`), provides TUN with auto-failover
- Templates:
  - `config/client/sing-box-auto.template.json` — main client config (urltest + Tailscale)
  - `config/client/xray-xhttp.template.json` — xray-core SOCKS proxy config
- Configs rendered to `~/.config/sing-box/config-auto.json` and `~/.config/xray/config.json`

### Traffic Routing
- Tailscale peers (100.x.x.x) → Tailscale endpoint
- Corporate subnets (10.0.0.0/8, 172.16.0.0/12) → Tailscale endpoint
- Russian domains/IPs (.ru, geoip-ru) → Direct (bypass VPN)
- Local network (192.168.x.x) → Direct
- Everything else → urltest auto-selects: XHTTP on 443 (via xray-core) or TCP+vision on 8443 (direct)

### Local State
- `users.json` maps UUIDs to friendly device names
- `.env` stores connection params (SERVER, PUBLIC_KEY, SHORT_ID, SNI, XHTTP_PATH, XHTTP_SNI, etc.)

### SSH-Based Operations
All server management happens via SSH. Scripts use `ssh $SSH_HOST` for commands and `scp` for file transfers.

### Key Scripts
- `scripts/deploy.sh` - Server hardening (UFW, SSH keys-only, unattended-upgrades), Docker install, REALITY key generation, container startup
- `scripts/xray-users` - User CRUD with UUID generation, config.json manipulation via `jq`, VLESS URL generation (both XHTTP and TCP+vision URLs)
- `scripts/generate-client-config` - Render client configs, package with scripts, create ZIP
- `scripts/render-config` - Renders both sing-box-auto and xray-xhttp templates using `envsubst`
- `scripts/validate-config` - Validate config via `sing-box check`
- `scripts/vpn-install` - Install sing-box + xray (via brew), configs, scripts, launchd plists, Finder shortcuts
- `scripts/vpn-start` - Install/update launchd plists (renders `${HOME}`), start xray-core + sing-box services
- `scripts/vpn-stop` - Unload launchd services, pkill fallback, remove search domain

### Security Layers
1. UFW firewall (ports 22, 443, 8443, 80)
2. SSH key-only authentication
3. Docker: `network_mode: host`, `read_only: true`, `no-new-privileges`
4. REALITY protocol encryption
5. XHTTP traffic fragmentation + random padding

## Required Local Tools

`bash`, `ssh`, `scp`, `jq`, `uuidgen`, `openssl`, `nc`, `envsubst`, `sing-box` (via brew), `xray` (via brew)

## Scripting Guidelines

- **Always use timeouts** - Every network operation, external command, or potentially blocking call must have a timeout to prevent scripts from hanging. Use `timeout <seconds> <command>` for commands, or built-in timeouts for tools like `curl -m`, `nc -w`, etc.

## Config Change Safety

- **NEVER restart apps/services before validating config** - Always test configuration syntax/validity BEFORE restarting. A bad config can break VPN connectivity and lock you out.
- For sing-box: use `sing-box check -c config.json` or `./scripts/validate-config` before restart
- For XRay: use `xray -test -config config.json` before container restart
- General rule: read the generated/merged config file and verify your changes are present and correct

## Files That Should Never Be Committed

`.env`, `users.json` - contain secrets and user UUIDs
