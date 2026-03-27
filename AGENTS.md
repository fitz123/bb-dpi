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
NAME=my-server SSH_HOST=host SERVER=ip make deploy  # Deploy new server
NAME=my-server make deploy                           # Redeploy existing server
make update              # Update XRay on all servers
make verify              # Health check all servers
make backup              # Backup config and secrets
make list                # List users

# User management (operates on ALL servers)
./scripts/xray-users add "Device Name"
./scripts/xray-users url "Device Name"    # Outputs URLs for all servers
./scripts/xray-users remove "Device Name"
./scripts/xray-users sync    # Sync local names with server
```

### Client Management
```bash
# Render both configs from skeletons + servers.json (dynamic N-server support)
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
- urltest outbound probes all server transports every 30s with `interrupt_exist_connections`
- xray-core runs as launchd service (`com.xray-xhttp`), provides SOCKS proxies (port 1080+i per server) for XHTTP
- sing-box runs as launchd service (`com.sing-box-vpn`), provides TUN with auto-failover
- Skeletons (static structure) + `servers.json` (server list) → `render-config` generates full configs via jq:
  - `config/client/sing-box-skeleton.json` — urltest, DNS, routes, Tailscale (no server outbounds)
  - `config/client/xray-xhttp-skeleton.json` — base structure (no inbounds/outbounds)
- Adding a server = one new object in `servers.json`, re-render. Zero template changes.
- Configs rendered to `~/.config/sing-box/config-auto.json` and `~/.config/xray/config.json`

### Traffic Routing
- Tailscale peers (100.x.x.x) → Tailscale endpoint
- Corporate subnets (10.0.0.0/8, 172.16.0.0/12) → Tailscale endpoint
- Russian domains/IPs (.ru, geoip-ru) → Direct (bypass VPN)
- Local network (192.168.x.x) → Direct
- Everything else → urltest auto-selects best transport across all servers (XHTTP on 443 via xray-core, or TCP+vision on 8443 direct)

### Local State
- `servers.json` — array of server objects (host, ssh, keys, paths). Adding a server = append one object.
- `users.json` — maps UUIDs to friendly device names
- `.env` — shared params only (FINGERPRINT, FLOW, TAILSCALE_*, COMPANY_DOMAIN). No per-server data.

### SSH-Based Operations
All server management happens via SSH. Scripts iterate `servers.json` for multi-server operations.

### Key Scripts
- `scripts/deploy.sh` - Deploy a named server: hardening, Docker, REALITY keys, saves to `servers.json`
- `scripts/xray-users` - User CRUD across all servers, VLESS URL generation for all servers
- `scripts/generate-client-config` - Render client configs, package with scripts + servers.json, create ZIP
- `scripts/render-config` - Builds configs from skeletons + `servers.json` via jq (envsubst for shared vars)
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

`.env`, `users.json`, `servers.json` - contain secrets, user UUIDs, and server private keys
