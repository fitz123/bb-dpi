# macOS Client Setup (sing-box)

This guide covers setting up XRay VLESS REALITY VPN on macOS using sing-box CLI.

> See also: [Client Configurations](client-configs.md) for Clash Verge and Linux setups.

## Why sing-box?

GUI apps like v2raytun and Hiddify have issues on macOS:
- Sandboxing prevents proper config import
- REALITY handshake failures
- Routing loops with TUN interfaces

sing-box CLI works reliably and can be fully automated.

## Installation

### 1. Download sing-box

```bash
# Download latest release (arm64 for Apple Silicon, amd64 for Intel)
curl -sL https://github.com/SagerNet/sing-box/releases/download/v1.11.0/sing-box-1.11.0-darwin-arm64.tar.gz | tar xz -C /tmp

# Install binary
sudo mv /tmp/sing-box-*/sing-box /usr/local/bin/
chmod +x /usr/local/bin/sing-box

# Verify installation
sing-box version
```

### 2. Create config directory

```bash
mkdir -p ~/.config/sing-box
```

### 3. Create configuration file

Create `~/.config/sing-box/config.json`:

```json
{
  "log": {
    "level": "info"
  },
  "dns": {
    "servers": [
      {
        "address": "https://1.1.1.1/dns-query",
        "detour": "proxy"
      },
      {
        "address": "local",
        "detour": "direct"
      }
    ]
  },
  "inbounds": [
    {
      "type": "tun",
      "address": ["172.19.0.1/30"],
      "auto_route": true,
      "strict_route": false,
      "stack": "system"
    }
  ],
  "outbounds": [
    {
      "type": "vless",
      "tag": "proxy",
      "server": "SERVER_IP",
      "server_port": 443,
      "uuid": "YOUR_UUID",
      "flow": "xtls-rprx-vision",
      "bind_interface": "en0",
      "tls": {
        "enabled": true,
        "server_name": "dl.google.com",
        "utls": {
          "enabled": true,
          "fingerprint": "chrome"
        },
        "reality": {
          "enabled": true,
          "public_key": "YOUR_PUBLIC_KEY",
          "short_id": "YOUR_SHORT_ID"
        }
      }
    },
    {
      "type": "direct",
      "tag": "direct"
    }
  ],
  "route": {
    "auto_detect_interface": true,
    "rules": [
      {
        "ip_is_private": true,
        "outbound": "direct"
      }
    ]
  }
}
```

**Important settings:**
- `bind_interface: "en0"` - Forces VPN traffic to use physical interface, preventing routing loops
- `strict_route: false` - Allows more flexible routing
- `auto_detect_interface: true` - Automatically detects default network interface

### 4. Validate configuration

```bash
sing-box check -c ~/.config/sing-box/config.json
```

## Desktop Shortcuts

### VPN-Start.command

Create `~/Desktop/VPN-Start.command`:

```bash
#!/bin/bash
echo Starting XRay VPN...
sudo /usr/local/bin/sing-box run -c ~/.config/sing-box/config.json
```

Make executable:
```bash
chmod +x ~/Desktop/VPN-Start.command
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
chmod +x ~/Desktop/VPN-Stop.command
```

## Usage

1. **Start VPN**: Double-click `VPN-Start.command` on Desktop
   - Enter sudo password when prompted
   - Terminal window stays open showing logs

2. **Stop VPN**: Double-click `VPN-Stop.command` on Desktop
   - Or press Ctrl+C in the terminal running sing-box
   - Or run `sudo pkill sing-box`

3. **Verify VPN is working**:
   ```bash
   curl ifconfig.me
   # Should show the VPN server IP
   ```

## Troubleshooting

### "can't assign requested address" errors

The VPN server IP is being routed through the TUN interface (routing loop).

**Fix**: Ensure `bind_interface: "en0"` is set in the proxy outbound config.

### Connection timeouts

1. Check server is reachable: `nc -zv SERVER_IP 443`
2. Verify config values match server (UUID, public key, short ID)
3. Check sing-box logs in terminal

### VPN connects but no internet

1. Check routing table: `netstat -rn | head -20`
2. Verify DNS is working: `nslookup google.com`
3. Try disabling strict_route in config

### Permission denied

sing-box needs root to create TUN interface:
```bash
sudo /usr/local/bin/sing-box run -c ~/.config/sing-box/config.json
```

## Network Interface Notes

- `en0` - Usually WiFi on MacBooks
- `en1` - Usually Ethernet (if available)
- Check your interface: `ifconfig | grep -E "^en[0-9]"`

If using Ethernet instead of WiFi, change `bind_interface` to match.

## Configuration Reference

| Field | Description |
|-------|-------------|
| `server` | XRay server IP address |
| `server_port` | Server port (usually 443) |
| `uuid` | Your client UUID |
| `public_key` | REALITY public key from server |
| `short_id` | REALITY short ID from server |
| `server_name` | SNI for REALITY (e.g., dl.google.com) |
| `bind_interface` | Physical NIC to bypass TUN routing |

## Getting Your Config Values

On the server, run:
```bash
./scripts/xray-users url "YourName"
```

This outputs a VLESS URL containing all required values.

## Quick Setup Script

Use the helper script to generate a complete config:

```bash
# Generate config for a user
./scripts/generate-client-config "device-name"

# Or from a VLESS URL
./scripts/generate-client-config "vless://uuid@host:port?..."
```

This creates:
- `config/client/generated/<name>-config.json` - Ready-to-use config file

Then copy to client:
```bash
scp config/client/generated/leo-mac-config.json user@client:~/.config/sing-box/config.json
scp config/client/VPN-*.command user@client:~/Desktop/
ssh user@client "chmod +x ~/Desktop/VPN-*.command"
```
