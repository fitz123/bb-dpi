# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

XRay REALITY VPN deployment infrastructure - automated setup and management of XRay VLESS REALITY VPN servers with security hardening. Pure Bash scripts with no build process.

## Commands

```bash
# First-time server deployment (SSH hardening, Docker install, XRay config)
make deploy

# Update XRay to latest version
make update

# Health check (port 443 connectivity + container status)
make verify

# Backup config and secrets
make backup

# List users
make list

# User management
./scripts/xray-users add "Device Name"
./scripts/xray-users url "Device Name"
./scripts/xray-users remove "Device Name"
./scripts/xray-users sync    # Push local users.json to server
```

## Architecture

### Dual-Layer Configuration
- **Server side**: XRay config.json with UUIDs in `/xray/config/config.json`
- **Local side**: `users.json` maps UUIDs to friendly names, `.env` stores connection params
- Templates in `config/server.template.json` get variable substitution at deploy time

### SSH-Based Operations
All server management happens via SSH. **Always use `ssh-xray` as the SSH host** for connecting to the XRay server. Scripts use `ssh $SSH_HOST` for commands and `scp` for file transfers.

### Key Scripts
- `scripts/deploy.sh` - Server hardening (UFW, SSH keys-only, unattended-upgrades), Docker install, REALITY key generation, container startup
- `scripts/xray-users` - User CRUD with UUID generation, config.json manipulation via `jq`, VLESS URL generation
- `scripts/update.sh` - Pull latest image, backup, restart
- `scripts/verify.sh` - Port check + container health

### Security Layers
1. UFW firewall (ports 22, 443 only)
2. SSH key-only authentication
3. Docker: `cap_drop: ALL`, `read_only: true`, `no-new-privileges: true`
4. REALITY protocol encryption

## Required Local Tools

`bash`, `ssh`, `scp`, `jq`, `uuidgen`, `openssl`, `nc`

## Files That Should Never Be Committed

`.env`, `users.json` - contain secrets and user UUIDs
