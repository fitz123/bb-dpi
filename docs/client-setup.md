# Client Setup (macOS)

XRay VLESS REALITY VPN with embedded Tailscale using sing-box.

## Quick Start

```bash
# Generate client package
./scripts/generate-client-config "device-name" "tskey-auth-xxx"

# Install on client
cd config/client/generated/device-name
./install.sh

# Quit Tailscale.app first, then start VPN
~/VPN-Start.command
```

**Note:** Wait ~15 seconds after startup for Tailscale to initialize.

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
# Should show VPN server IP
curl ifconfig.me

# Should show real IP (direct)
curl 2ip.ru
```

## User Management

```bash
./scripts/xray-users list              # List users
./scripts/xray-users add "Device"      # Add user
./scripts/xray-users url "Device"      # Get URL
./scripts/xray-users remove "Device"   # Remove user
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
