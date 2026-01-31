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
- **Embedded Tailscale** - Corporate network access without standalone Tailscale app
- **Russia bypass** - Russian sites go direct (no VPN) for DPI evasion
- **Smart DNS routing** - Split DNS for different domain types

## DNS Architecture

| Server | Type | Domains | Notes |
|--------|------|---------|-------|
| `magicdns` | Native Tailscale | `.ts.net` | Tailscale peer hostnames |
| `company-dns` | UDP <INTERNAL_DNS_1> | `.<COMPANY_DOMAIN>` | Internal DNS via Tailscale routing |
| `company-dns-fallback` | UDP <INTERNAL_DNS_2> | (backup) | Available if primary fails |
| `russia-dns` | UDP 77.88.8.8 | `.ru`, `.su`, etc. | Yandex DNS, direct |
| `proxy-dns` | HTTPS 1.1.1.1 | Everything else | Cloudflare DoH via VPN |

**Note:** To use fallback DNS, edit `config.json` and change `"server": "company-dns"` to `"server": "company-dns-fallback"`.

**Important:** Embedded Tailscale needs ~15 seconds after startup to initialize. DNS queries for corporate/Tailscale domains will fail until then.

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

| Destination | DNS Server | Outbound |
|-------------|------------|----------|
| `.ts.net` (Tailscale hostnames) | `magicdns` (native) | Tailscale endpoint |
| `.<COMPANY_DOMAIN>` | `company-dns` (<INTERNAL_DNS_1>) | Tailscale endpoint |
| `.ru`, `.su`, Russian sites | `russia-dns` (77.88.8.8) | Direct |
| Private IPs | - | Direct |
| Tailscale advertised routes | - | Tailscale endpoint (`preferred_by`) |
| Everything else | `proxy-dns` (1.1.1.1) | XRay VPN |

## Embedded Tailscale

The template includes an embedded Tailscale endpoint that provides:
- **MagicDNS** for `.ts.net` domains (native tailscale DNS type)
- **Routing** to Tailscale-advertised subnets via `preferred_by` rule
- **No standalone app needed** - quit Tailscale.app before starting VPN
- **Automatic authentication** via `auth_key`

### Startup Behavior

1. sing-box starts and creates TUN interface
2. Embedded Tailscale connects to control plane (~5-15 seconds)
3. Once IPv4 is assigned, DNS and routing become available

**Note:** DNS queries for `.ts.net` or `.<COMPANY_DOMAIN>` will fail with "missing Tailscale IPv4 address" until Tailscale initializes (~15 seconds after startup).

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
./local/scripts/network-diag.sh
```

Expected results:
- MagicDNS: `vm-msk-tailscale-1.<TS_TAILNET> → 100.x.x.x`
- Corporate DNS: `git.<COMPANY_DOMAIN> → 10.x.x.x`
- Russia bypass: `yandex.ru → real IP (not 198.18.x.x)`
- VPN exit: `curl ifconfig.me → <SERVER_IP>`
- SSH test: `ssh <INTERNAL_SERVER> → vm-msk-gitea-1`

**Note:** Wait ~15 seconds after starting sing-box before running diagnostics.

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

### "missing Tailscale IPv4 address" errors

Embedded Tailscale hasn't initialized yet.

**Fix**: Wait ~15 seconds after startup for Tailscale to get its IPv4 assigned.

### "can't assign requested address" errors

The VPN server IP is being routed through the TUN interface (routing loop).

**Fix**: Ensure `route_exclude_address` includes the VPN server IP in the inbound config.

### Connection timeouts

1. Check server is reachable: `nc -zv SERVER_IP 443`
2. Verify config values match server (UUID, public key, short ID)
3. Check sing-box logs

### Corporate DNS not resolving

1. **Wait 15 seconds** after startup for Tailscale to initialize
2. Verify Tailscale routing works: `nc -z <INTERNAL_DNS_1> 53` (should succeed)
3. Check DNS server is correct in config: should be `<INTERNAL_DNS_1>` (NOT `100.100.100.100`)

**Note:** MagicDNS IP `100.100.100.100` does NOT work via embedded Tailscale. Use internal DNS servers instead.

### .ts.net domains not resolving

1. **Wait 15 seconds** for Tailscale to initialize
2. Verify `magicdns` server is configured with `"type": "tailscale"`
3. Check DNS rule routes `.ts.net` to `magicdns` server

### Standalone Tailscale conflicts

**Fix**: Quit Tailscale.app before starting sing-box:
```bash
osascript -e 'quit app "Tailscale"'
```

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
