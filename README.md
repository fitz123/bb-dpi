# XRay REALITY VPN

Clean deployment of XRay VLESS REALITY VPN server.

## Quick Start

```bash
# First time deployment
make deploy

# Verify connection
make verify

# List users
make list

# Add user
./scripts/xray-users add "Device Name"
```

## Architecture

- **Protocol**: VLESS + REALITY + Vision
- **Transport**: TCP on port 443
- **SNI**: dl.google.com
- **Container**: `ghcr.io/xtls/xray-core:latest`

## Server Hardening

The deploy script automatically configures:
- UFW firewall (ports 22, 443 only)
- SSH key-only authentication
- Automatic security updates (unattended-upgrades)

## Docker Security

Container runs with:
- `cap_drop: ALL`
- `read_only: true`
- `no-new-privileges: true`
- Log rotation (10MB max, 3 files)

## Files

```
.env.example    - Configuration template
.env            - Your config (git-ignored)
users.json      - User name mapping (git-ignored)
docker-compose.yml - Container definition
config/server.template.json - XRay config template
scripts/
  deploy.sh     - First-time deployment
  xray-users    - User management CLI
  update.sh     - Update XRay version
  backup.sh     - Backup config
  verify.sh     - Health check
```

## User Management

```bash
# Add user and get share URL
./scripts/xray-users add "Mom iPhone"

# Get URL for existing user
./scripts/xray-users url "Mom iPhone"

# Remove user
./scripts/xray-users remove "Mom iPhone"

# Sync names with server
./scripts/xray-users sync
```

## Maintenance

```bash
# Update XRay to latest
make update

# Manual backup
make backup

# Check server health
make verify
```

## Clash Profile

After deployment, update your Clash profile:
- `server`: your server IP
- `uuid`: from share URL
- `reality-opts.public-key`: from .env
- `reality-opts.short-id`: from .env

## Credits

Built with assistance from [Claude Code](https://claude.ai/code) (Anthropic) and [Codex](https://openai.com/codex) (OpenAI).
