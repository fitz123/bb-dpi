# Client Setup (macOS)

XRay VLESS REALITY VPN with auto-failover. By default the Mac is a **thin VLESS client** — corporate (`10.x`, `*.<COMPANY_DOMAIN>`) and tailnet (`100.64.x`) access is delegated to a VPN exit running its own Tailscale node. `--with-tailscale` switches to embedded tsnet on the Mac if you want a per-laptop tailnet identity.

## Quick Start

```bash
# Generate client package
./scripts/generate-client-config "device-name"

# Install on client (in the generated package directory)
cd config/client/generated/device-name
./vpn-install

# Start
vpn-start --with-corp-dns        # render with corp DNS, then start
```

Default configuration: no embedded Tailscale, corp DNS resolved through the VLESS chain when `--with-corp-dns` is set.

## Render flags

```bash
./scripts/render-config                              # bare VLESS, no corp/tailnet
./scripts/render-config --with-corp-dns              # + corp DNS via VLESS exit
./scripts/render-config --with-tailscale             # + embedded tsnet (no corp DNS)
./scripts/render-config --with-tailscale --with-corp-dns  # legacy: corp DNS via tsnet
./scripts/render-config --proto tcp-vision           # skip xray-core (TCP+vision only)
```

`vpn-start` forwards all flags verbatim to `render-config`. `vpn-start` with no args just (re)starts the existing rendered config.

## Architectures at a glance

### Default — Tailscale on the VPN exit (recommended)
```
Mac apps → sing-box TUN → VLESS+REALITY → xray on VPN exit → tailnet (via exit's Tailscale)
```
- Mac doesn't have a tailnet identity. ACL on tailnet only sees the VPN exit.
- One auth/tag on the exit handles all your laptops.
- Bandwidth and latency hop through the exit, plus one Tailscale hop to the corp subnet router.
- Corp DNS (`${INTERNAL_DNS_1}` in `.env`) is queried through the VLESS chain when `--with-corp-dns` is set (sing-box's `company-dns` server with `detour: auto`).

Provisioning the VPN exit:
```bash
# On the exit:
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up --accept-routes --hostname=<vpn-exit-name> --auth-key=tskey-...
```
**`--accept-routes` is required**, not optional. Without it the kernel has no route for corp subnets and xray's `freedom` outbound dumps `10.x` traffic onto the default gateway, where it dies. Verify after `up`:
```bash
ip route | grep -E '10\.|100\.6'   # should list corp subnets via tailscale0
sysctl net.ipv4.ip_forward         # must be 1
sudo iptables -t nat -L POSTROUTING -n   # MASQUERADE chain expected
```
In the Tailscale admin UI, tag the exit so it carries the ACL access you need (e.g. `tag:superadmin`). The auth key may pre-tag if it was minted with the right tag attribute.

### Opt-in — embedded Tailscale on the Mac (`--with-tailscale`)
```
Mac apps → sing-box TUN → embedded tsnet → tailnet (direct)
                       ↓ (everything else)
                       VLESS → VPN exit → public internet
```
- Per-Mac tailnet identity (`sing-box-mac` or whatever `TAILSCALE_HOSTNAME` is set to in `.env`).
- Useful if you want admin-UI per-device visibility, or you can't put Tailscale on the exit.
- Subject to per-laptop NAT/DERP edge cases (see `docs/adr-001-...` if/when written about hel-DERP findings).

## Manual single-host setup (no client package)

```bash
brew install sing-box xray jq
mkdir -p ~/.config/sing-box ~/.config/xray
git clone <this-repo> ~/bb-dpi && cd ~/bb-dpi
# Populate .env, servers.json, users.json (see README)
./scripts/render-config --with-corp-dns      # or whatever flag combo you want
./scripts/validate-config                    # sing-box check
sudo cp config/client/com.{xray-xhttp,sing-box-vpn}.plist /Library/LaunchDaemons/
vpn-start
```

## Testing

```bash
# General internet via VPN — should show the VPN exit's IP
curl -sm 4 https://api.ipify.org

# Russian sites — should bypass VPN (direct)
curl -sm 4 https://2ip.ru

# Corp DNS (with --with-corp-dns) — pick any host in your tailnet
dig +short <some-host>.${COMPANY_DOMAIN} @${INTERNAL_DNS_1}

# Corp host reachability (TCP only — sing-box's auto outbound doesn't pass ICMP)
nc -zv ${INTERNAL_DNS_1} 53
nc -zv <corp-host-tailnet-ip> 22
```

`ping` to corp/tailnet hosts will time out even when the path works — sing-box's urltest `auto` outbound doesn't carry ICMP. Use `nc -zv` or actual TCP traffic to test.

## User Management

```bash
./scripts/xray-users list              # List users
./scripts/xray-users add "Device"      # Add user, prints share URLs
./scripts/xray-users url "Device"      # Get URLs
./scripts/xray-users remove "Device"
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Corp DNS not resolving | Re-render with `--with-corp-dns`. If still failing, verify the exit can resolve from its own shell: `dig @${INTERNAL_DNS_1} <host>` |
| `dig` fails but `nc -zv <corp-dns> 53` works | `--with-corp-dns` not applied — `dig +short ... @<corp-dns>` is being intercepted by sing-box's DNS hijack, which has no `company-dns` server to forward to |
| Corp IP unreachable through VLESS | On the VPN exit: confirm `tailscale status` shows it active and `ip route` lists corp subnets via `tailscale0`. Most common cause: `--accept-routes` was omitted at `tailscale up` time |
| Connection timeouts to VPN server | Server reachable on 443? `nc -zv $SERVER_IP 443` |
| Permission denied | `vpn-start` and `vpn-stop` need sudo for launchctl; the wrapper handles it |
| `--with-tailscale` mode: "missing Tailscale IPv4 address" | Wait ~15s after start for tsnet to register |
| `--with-tailscale` mode: peer unreachable | DERP-region quirks; see `tailscale netcheck` and per-peer status. The Tailscale-on-exit architecture sidesteps this |

## Re-authenticate Tailscale (only relevant for `--with-tailscale`)

```bash
sudo rm -rf ~/.local/share/sing-box-tailscale/*
vpn-stop && vpn-start --with-tailscale --with-corp-dns
```
