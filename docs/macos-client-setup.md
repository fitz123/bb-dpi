# macOS Client Setup (sing-box)

Quick setup for XRay VLESS REALITY VPN with embedded Tailscale on macOS.

> Full configuration details: [Client Configurations](client-configs.md)

## Quick Start

```bash
# Generate client package (run from project root)
./scripts/generate-client-config "device-name" "tskey-auth-xxx"

# Install on client
cd config/client/generated/device-name
./install.sh

# Quit Tailscale.app first, then start VPN
~/VPN-Start.command
```

**Note:** Wait ~15 seconds after startup for Tailscale to initialize before testing corporate DNS.

## Manual Setup

```bash
# Install sing-box
brew install sing-box

# Create directories
mkdir -p ~/.config/sing-box ~/.local/share/sing-box-tailscale

# Generate config from template
source .env
envsubst < config/client/sing-box.template.json > ~/.config/sing-box/config.json

# Validate and run
sing-box check -c ~/.config/sing-box/config.json
sudo sing-box run -c ~/.config/sing-box/config.json
```

## Desktop Shortcuts

Create `~/Desktop/VPN-Start.command`:
```bash
#!/bin/bash
echo Starting XRay VPN...
sudo sing-box run -c ~/.config/sing-box/config.json
```

Create `~/Desktop/VPN-Stop.command`:
```bash
#!/bin/bash
sudo pkill sing-box
echo VPN stopped.
```

Make executable: `chmod +x ~/Desktop/VPN-*.command`

## Testing

```bash
./local/scripts/network-diag.sh
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "missing Tailscale IPv4 address" | Wait ~15 seconds for Tailscale to initialize |
| Corporate DNS not resolving | Verify Tailscale routing to internal DNS |
| Connection timeouts | Check server reachable: `nc -zv $SERVER_IP 443` |
| Permission denied | Run with `sudo` |
| Tailscale conflicts | Quit Tailscale.app before starting |

## Re-authenticate Tailscale

```bash
sudo rm -rf ~/.local/share/sing-box-tailscale/*
# Restart sing-box
```
