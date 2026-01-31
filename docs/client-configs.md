# Client Configurations

VPN client configurations for XRay REALITY server with Russian domain bypass.

## Routing Strategy

- **VPN**: International traffic
- **Direct**: Russian domains/IPs, private networks, local domains

## Connection Parameters

All values are in `.env` file. Generate client URL with:
```bash
./scripts/xray-users url "<device-name>"
```

---

## sing-box (macOS with Tailscale)

See [macOS Client Setup](macos-client-setup.md) for installation.

**Features:**
- Embedded Tailscale for corporate network access
- MagicDNS for `.ts.net` domains
- Corporate DNS via Tailscale routing

---

## Clash Verge (macOS/Windows)

**Location**: `~/.config/clash-verge-rev/` or via GUI

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
    servername: <SNI_DOMAIN>
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
  - RULE-SET,russia-domains,DIRECT
  - DOMAIN-SUFFIX,ru,DIRECT
  - DOMAIN-SUFFIX,su,DIRECT
  - DOMAIN-SUFFIX,xn--p1ai,DIRECT
  - GEOIP,RU,DIRECT
  - MATCH,Proxy
```

---

## sing-box (Linux)

**Location**: `~/.config/sing-box/config.json`

```json
{
  "log": { "level": "info", "timestamp": true },
  "dns": {
    "servers": [
      { "tag": "cloudflare", "type": "https", "server": "1.1.1.1" },
      { "tag": "yandex", "type": "udp", "server": "77.88.8.8" },
      { "tag": "local", "type": "local" }
    ],
    "rules": [
      { "server": "local", "domain_suffix": [".local", ".localhost", ".lan"] },
      { "server": "yandex", "domain_suffix": [".ru", ".su", ".xn--p1ai"] }
    ],
    "final": "cloudflare",
    "strategy": "ipv4_only"
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "singbox",
      "address": ["172.19.0.1/30"],
      "mtu": 1400,
      "auto_route": true,
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
      "tls": {
        "enabled": true,
        "server_name": "<SNI_DOMAIN>",
        "utls": { "enabled": true, "fingerprint": "chrome" },
        "reality": {
          "enabled": true,
          "public_key": "<PUBLIC_KEY>",
          "short_id": "<SHORT_ID>"
        }
      }
    },
    { "type": "direct", "tag": "direct" }
  ],
  "route": {
    "auto_detect_interface": true,
    "final": "proxy",
    "rules": [
      { "action": "sniff" },
      { "protocol": "dns", "action": "hijack-dns" },
      { "ip_is_private": true, "outbound": "direct" },
      { "domain_suffix": [".ru", ".su", ".xn--p1ai"], "outbound": "direct" }
    ]
  }
}
```

### Linux Installation (Fedora/Bazzite)

```bash
mkdir -p ~/.local/bin ~/.config/sing-box
curl -sL https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-*-linux-amd64.tar.gz | tar xz -C /tmp
mv /tmp/sing-box-*/sing-box ~/.local/bin/
sudo setcap 'cap_net_admin,cap_net_bind_service,cap_net_raw+ep' ~/.local/bin/sing-box

~/.local/bin/sing-box run -c ~/.config/sing-box/config.json
```

---

## VLESS Share URL

For mobile clients (v2rayNG, Shadowrocket):

```
vless://<UUID>@<SERVER>:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=<SNI>&fp=chrome&pbk=<PUBLIC_KEY>&sid=<SHORT_ID>&type=tcp#<NAME>
```

Generate: `./scripts/xray-users url "<device-name>"`

---

## Testing

```bash
# Should show VPN server IP
curl ifconfig.me

# Should show real IP (direct connection)
curl 2ip.ru
```

## User Management

```bash
./scripts/xray-users list              # List users
./scripts/xray-users add "Device"      # Add user
./scripts/xray-users url "Device"      # Get URL
./scripts/xray-users remove "Device"   # Remove user
```
