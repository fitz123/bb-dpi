# Client Configurations

VPN client configurations for connecting to the XRay REALITY server with Russian domain bypass.

## Routing Strategy

All configurations implement the same routing logic:
- **VPN**: International traffic routes through the VPN server
- **Direct**: Russian domains/IPs bypass VPN for optimal latency
- **Direct**: Private/local networks always bypass VPN

### Bypass Rules (Direct)
- Russian TLDs: `.ru`, `.su`, `.xn--p1ai` (рф)
- Russian services: `yandex.com`, `yandex.net`, `vk.com`, `mail.ru`, `ok.ru`, `t.me`
- Private IPs: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`
- Local domains: `.local`, `.localhost`, `.lan`, `.home`, `.internal`
- VPN server subnet: `<SERVER_SUBNET>`

### DNS Strategy
- International domains: Cloudflare DoH (`1.1.1.1`)
- Russian domains: Yandex DNS (`77.88.8.8`)
- Local domains: System resolver

---

## Clash Verge (macOS/Windows)

**Location**: `~/.config/clash-verge-rev/` or via Clash Verge GUI

```yaml
mixed-port: 7890
mode: rule
log-level: info
ipv6: false

geodata-mode: true
geo-auto-update: true
geo-update-interval: 24

dns:
  enable: true
  ipv6: false
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  fake-ip-filter:
    - "*.lan"
    - "*.local"
  nameserver:
    - https://1.1.1.1/dns-query
    - https://8.8.8.8/dns-query
  nameserver-policy:
    "+.ru": [system, 77.88.8.8, 77.88.8.1]
    "+.su": [system, 77.88.8.8, 77.88.8.1]
    "+.xn--p1ai": [system, 77.88.8.8, 77.88.8.1]
    "+.yandex.com": [77.88.8.8, 77.88.8.1]
    "+.yandex.net": [77.88.8.8, 77.88.8.1]
    "+.vk.com": [77.88.8.8, 77.88.8.1]
    "+.t.me": [77.88.8.8, 77.88.8.1]

tun:
  enable: true
  stack: gvisor
  auto-route: true
  auto-detect-interface: true
  dns-hijack:
    - any:53

proxies:
  - name: XRay-Reality
    type: vless
    server: <SERVER_IP>
    port: 443
    uuid: <YOUR_UUID>
    network: tcp
    udp: true
    tls: true
    flow: xtls-rprx-vision
    servername: dl.google.com
    reality-opts:
      public-key: <PUBLIC_KEY>
      short-id: <SHORT_ID>
    client-fingerprint: chrome

proxy-groups:
  - name: Proxy
    type: select
    proxies:
      - XRay-Reality
      - DIRECT

rule-providers:
  russia-domains:
    type: http
    behavior: domain
    url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/category-ru.list"
    path: ./ruleset/russia-domains.list
    interval: 86400

rules:
  - GEOIP,private,DIRECT
  - GEOSITE,private,DIRECT
  - IP-CIDR,<SERVER_SUBNET>,DIRECT
  - RULE-SET,russia-domains,DIRECT
  - DOMAIN-SUFFIX,ru,DIRECT
  - DOMAIN-SUFFIX,su,DIRECT
  - DOMAIN-SUFFIX,xn--p1ai,DIRECT
  - GEOIP,RU,DIRECT
  - MATCH,Proxy
```

---

## sing-box (Linux/Bazzite)

**Version**: 1.12.x (uses new DNS server format)

**Location**: `~/.config/sing-box/config.json`

```json
{
  "log": {
    "level": "info",
    "timestamp": true
  },
  "dns": {
    "servers": [
      {
        "tag": "cloudflare",
        "type": "https",
        "server": "1.1.1.1"
      },
      {
        "tag": "yandex",
        "type": "udp",
        "server": "77.88.8.8"
      },
      {
        "tag": "local",
        "type": "local"
      }
    ],
    "rules": [
      {
        "server": "local",
        "domain_suffix": [".local", ".localhost", ".lan", ".home", ".internal"]
      },
      {
        "server": "yandex",
        "domain_suffix": [".ru", ".su", ".xn--p1ai", ".yandex.com", ".yandex.net", ".vk.com", ".mail.ru", ".ok.ru", ".t.me"]
      }
    ],
    "final": "cloudflare",
    "strategy": "ipv4_only"
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "singbox",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "mtu": 1400,
      "auto_route": true,
      "strict_route": false,
      "stack": "gvisor"
    }
  ],
  "outbounds": [
    {
      "type": "vless",
      "tag": "proxy",
      "server": "<SERVER_IP>",
      "server_port": 443,
      "uuid": "<YOUR_UUID>",
      "flow": "xtls-rprx-vision",
      "domain_resolver": "local",
      "tls": {
        "enabled": true,
        "server_name": "dl.google.com",
        "utls": {
          "enabled": true,
          "fingerprint": "chrome"
        },
        "reality": {
          "enabled": true,
          "public_key": "<PUBLIC_KEY>",
          "short_id": "<SHORT_ID>"
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
    "default_domain_resolver": "local",
    "final": "proxy",
    "rules": [
      {
        "action": "sniff"
      },
      {
        "protocol": "dns",
        "action": "hijack-dns"
      },
      {
        "ip_is_private": true,
        "outbound": "direct"
      },
      {
        "domain_suffix": [".local", ".localhost", ".lan", ".home", ".internal"],
        "outbound": "direct"
      },
      {
        "ip_cidr": ["127.0.0.0/8", "::1/128", "<SERVER_SUBNET>"],
        "outbound": "direct"
      },
      {
        "domain_suffix": [".ru", ".su", ".xn--p1ai"],
        "outbound": "direct"
      },
      {
        "domain_suffix": [".yandex.com", ".yandex.net", ".vk.com", ".mail.ru", ".ok.ru", ".t.me"],
        "outbound": "direct"
      }
    ]
  }
}
```

### sing-box Installation (Bazzite/Fedora)

```bash
# Download and install
mkdir -p ~/.local/bin ~/.config/sing-box
curl -sL https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-*-linux-amd64.tar.gz \
  | tar xz -C /tmp
mv /tmp/sing-box-*/sing-box ~/.local/bin/

# Set capabilities for TUN
sudo setcap 'cap_net_admin,cap_net_bind_service,cap_net_raw+ep' ~/.local/bin/sing-box

# Validate config
~/.local/bin/sing-box check -c ~/.config/sing-box/config.json

# Create systemd user service
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/sing-box.service << 'EOF'
[Unit]
Description=sing-box VPN service
After=network-online.target

[Service]
Type=simple
ExecStart=%h/.local/bin/sing-box run -c %h/.config/sing-box/config.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

# Enable and start
systemctl --user daemon-reload
systemctl --user enable --now sing-box
loginctl enable-linger $USER
```

### sing-box Commands

```bash
systemctl --user status sing-box     # Check status
systemctl --user restart sing-box    # Restart
systemctl --user stop sing-box       # Stop
journalctl --user -u sing-box -f     # Follow logs
```

---

## VLESS Share URL

For mobile clients (v2rayNG, Shadowrocket, etc.):

```
vless://<UUID>@<SERVER_IP>:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=dl.google.com&fp=chrome&pbk=<PUBLIC_KEY>&sid=<SHORT_ID>&type=tcp#<NAME>
```

Generate with:
```bash
./scripts/xray-users url "<device-name>"
```

---

## Connection Parameters

| Parameter | Value |
|-----------|-------|
| Server | `<SERVER_IP>` |
| Port | `443` |
| Protocol | VLESS |
| Flow | `xtls-rprx-vision` |
| Security | REALITY |
| SNI | `dl.google.com` |
| Fingerprint | `chrome` |
| Public Key | `<PUBLIC_KEY>` |
| Short ID | `<SHORT_ID>` |

---

## Testing

Verify split routing is working:

```bash
# Should show VPN IP (<SERVER_IP>)
curl ifconfig.me

# Should show your real IP (direct connection)
curl 2ip.ru

# Both should work
curl -I https://google.com    # via VPN
curl -I https://yandex.ru     # direct
```

---

## User Management

```bash
./scripts/xray-users list              # List all users
./scripts/xray-users add "New Device"  # Add new user
./scripts/xray-users url "Device"      # Get connection URL
./scripts/xray-users remove "Device"   # Remove user
```

**Note**: User UUIDs are stored in `users.json` (not committed to git).
