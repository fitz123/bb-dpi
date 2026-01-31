# macOS Client Setup (sing-box)

This guide covers setting up XRay VLESS REALITY VPN on macOS using sing-box CLI.

> See also: [Client Configurations](client-configs.md) for Clash Verge and Linux setups.

## Why sing-box?

GUI apps like v2raytun and Hiddify have issues on macOS:
- Sandboxing prevents proper config import
- REALITY handshake failures
- Routing loops with TUN interfaces

sing-box CLI works reliably and can be fully automated.

## Features

The sing-box template includes:
- **Embedded Tailscale** - MagicDNS for corporate domains without standalone Tailscale app
- **Russia bypass** - Russian sites go direct (no VPN) for DPI evasion
- **Smart DNS routing** - Corporate domains via Tailscale, Russian via Yandex, others via Cloudflare DoH

## Installation

### 1. Install sing-box via Homebrew

```bash
brew install sing-box

# Verify installation
sing-box version
```

### 2. Create config directory

```bash
mkdir -p ~/.config/sing-box
mkdir -p ~/.local/share/sing-box-tailscale
```

### 3. Generate configuration

Use the template with environment variable substitution:

```bash
# Load environment
source .env

# Generate config from template
envsubst < config/client/sing-box.template.json > ~/.config/sing-box/config.json

# Validate
sing-box check -c ~/.config/sing-box/config.json
```

### 4. Run sing-box

```bash
sudo sing-box run -c ~/.config/sing-box/config.json
```

## Traffic Routing

| Destination | DNS | Outbound |
|-------------|-----|----------|
| `.<COMPANY_DOMAIN>`, `.ts.net` | Tailscale MagicDNS | Direct |
| `.ru`, `.su`, Russian sites | Yandex (77.88.8.8) | Direct |
| Private IPs, Tailscale CGNAT | - | Direct |
| Everything else | Cloudflare DoH | XRay VPN |

## Embedded Tailscale

The template includes an embedded Tailscale endpoint that provides:
- MagicDNS resolution for corporate domains
- No need for standalone Tailscale app
- Automatic authentication via `auth_key`

### Getting a Tailscale Auth Key

1. Go to https://login.tailscale.com/admin/settings/keys
2. Generate an auth key (reusable recommended)
3. Add to `.env`: `TAILSCALE_AUTH_KEY="tskey-auth-..."`

### State Directory

Tailscale state is stored in `~/.local/share/sing-box-tailscale/`. To re-authenticate:

```bash
sudo rm -rf ~/.local/share/sing-box-tailscale/*
# Restart sing-box
```

## Testing

Run the diagnostic script:

```bash
sudo ./local/scripts/network-diag.sh
```

Expected results:
- Corporate DNS: `git.<COMPANY_DOMAIN> → 10.x.x.x`
- Russia bypass: `yandex.ru → real IP`
- VPN exit: `<SERVER_IP>`

## Desktop Shortcuts

### VPN-Start.command

Create `~/Desktop/VPN-Start.command`:

```bash
#!/bin/bash
echo Starting XRay VPN...
sudo sing-box run -c ~/.config/sing-box/config.json
```

### VPN-Stop.command

Create `~/Desktop/VPN-Stop.command`:

```bash
#!/bin/bash
echo Stopping XRay VPN...
sudo pkill sing-box
echo VPN stopped.
sleep 2
```

Make executable:
```bash
chmod +x ~/Desktop/VPN-*.command
```

## Troubleshooting

### "can't assign requested address" errors

The VPN server IP is being routed through the TUN interface (routing loop).

**Fix**: Ensure `route_exclude_address` includes the VPN server IP in the inbound config.

### Connection timeouts

1. Check server is reachable: `nc -zv SERVER_IP 443`
2. Verify config values match server (UUID, public key, short ID)
3. Check sing-box logs

### Corporate DNS not resolving

1. Check Tailscale endpoint is authenticated: `sudo cat ~/.local/share/sing-box-tailscale/tailscaled.state | jq keys`
2. Should show `["_current-profile", "_machinekey", "_profiles", "profile-xxxx"]`
3. If only `_machinekey`, re-authenticate by clearing state directory

### Permission denied

sing-box needs root to create TUN interface:
```bash
sudo sing-box run -c ~/.config/sing-box/config.json
```

## Configuration Reference

| Field | Description |
|-------|-------------|
| `server` | XRay server IP address |
| `server_port` | Server port (usually 443) |
| `uuid` | Your client UUID |
| `public_key` | REALITY public key from server |
| `short_id` | REALITY short ID from server |
| `server_name` | SNI for REALITY (e.g., dl.google.com) |
| `auth_key` | Tailscale auth key for embedded endpoint |

## Getting Your Config Values

On the server, run:
```bash
./scripts/xray-users url "YourName"
```

This outputs a VLESS URL containing all required values.
