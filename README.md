# XRay REALITY VPN

DPI-resistant VPN with auto-failover between XHTTP and TCP+vision transports.

## Quick Start

```bash
# First time deployment
make deploy

# Verify connection
make verify

# Add user and get share URLs
./scripts/xray-users add "Device Name"

# List users
make list
```

## Architecture

- **Server**: XRay VLESS + REALITY on Docker (`network_mode: host`). Two roles:
  - **Exit** (default) — XHTTP on port 443 (primary, DPI-resistant HTTP fragmentation), TCP+vision on port 8443 (fallback). `freedom` outbound exits to internet.
  - **Relay** — same two inbounds, but xray dual-role: each inbound chains via VLESS+REALITY to an **upstream exit server**. Set `relay_upstream: "<exit-name>"` in `servers.json`. Useful for routing around regional DPI: client → relay (different ISP/ASN) → upstream exit → internet. See [relay deployment](#relay-deployment) below.
  - **REALITY SNI choice**: pick a SNI hostname whose resolved IP is on the **same ASN** as the server's IP. Active probes to the REALITY server are forwarded raw to `dest`; an ASN mismatch (server in DC X claiming to host a site in DC Y) is detectable. Per-server SNI lives in `servers.json` (`xhttp_sni`, `sni`).
  - **Upstream-only servers**: set `"client_render": false` on a `servers.json` entry to deploy and `xray-users`-sync it normally, but hide it from client `render-config` output. Use this for relay-only exits that should never appear in client urltest pools directly. The flag defaults to `true` when absent — existing entries don't need changing.
- **Client**: sing-box TUN with urltest auto-failover
  - xray-core SOCKS proxy for XHTTP transport (port 1080+i per server)
  - sing-box native VLESS for TCP+vision fallback
- **Tailscale (corporate access)**: by default the Mac is a thin VLESS client and the **VPN exit** runs Tailscale. IP-level corporate traffic (`10.x`, `100.64.x`) tunnels through VLESS → exit server's xray → kernel routes via the exit's Tailscale interface → tailnet *by default*. **Hostname resolution** for `*.<COMPANY_DOMAIN>` requires `--with-corp-dns` at render time — without it, corp domains resolve via `1.1.1.1` (which has no internal records). The exit must run `tailscale up --accept-routes` (**required**, not optional — without it the kernel won't have the corp routes that xray's `freedom` outbound depends on) and be tagged with whatever ACL grants corp access. Opt into per-Mac embedded tsnet with `--with-tailscale` if you need per-laptop tailnet identity instead.
- **Auto-failover**: urltest probes both transports every 30s, instant switchover on failure

## Server Hardening

The deploy script automatically configures:
- UFW firewall (ports 22, 443, 8443, 80)
- SSH key-only authentication
- Automatic security updates (unattended-upgrades)

## Docker Security

Container runs with:
- `network_mode: host` (required for multi-port REALITY)
- `read_only: true`
- `no-new-privileges: true`
- Log rotation (10MB max, 3 files)

## Client Setup

```bash
# Render configs (default: no embedded Tailscale, no corp DNS — thin VLESS client)
./scripts/render-config

# Default + corp DNS resolves through the VLESS exit
./scripts/render-config --with-corp-dns

# Embed Tailscale on this Mac instead of relying on the VPN exit
./scripts/render-config --with-tailscale --with-corp-dns

# vpn-start forwards all flags verbatim to render-config when args are present
vpn-start                                    # use existing rendered configs
vpn-start --with-corp-dns                    # re-render then start
vpn-start --proto tcp-vision --with-corp-dns # any combination
vpn-stop
```

Two launchd services manage the client:
- `com.xray-xhttp` — xray-core SOCKS proxy (XHTTP on port 1080+i per server)
- `com.sing-box-vpn` — sing-box TUN with urltest auto-failover

`vpn-start` auto-detects whether xray is needed by inspecting the rendered
sing-box config (presence of any `xhttp-*` SOCKS outbound). With
`--proto tcp-vision`, xray is stopped and not relaunched.

### Tailscale on the VPN exit (default architecture)

On any VPN exit you want to use as a tailnet jumphost:

```bash
# On the exit server:
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up --accept-routes --hostname=<vpn-exit-name> --auth-key=tskey-...
# In Tailscale admin UI: tag this node so it has the corp ACL access you need.
```

**`--accept-routes` is required, not optional.** Without it tailscaled won't install the corporate subnet routes locally, so xray's `freedom` outbound has no kernel route for `10.x` traffic and corp packets fall through to the default gateway. Verify the routes after bringing tailscale up:

```bash
ip route | grep -E '10\.|100\.6'   # should list corp subnets via tailscale0
```

Also required (typically pre-set on a stock VPN host): `net.ipv4.ip_forward=1` and a default `MASQUERADE` rule. Verify with `iptables -t nat -L POSTROUTING -n`.

### Relay deployment

A relay server sits between clients and an existing exit server. Useful when direct client→exit traffic is being filtered (e.g., per-region DPI on cloud ASN ranges) but a different-network host can reach the exit cleanly. The relay terminates VLESS+REALITY from clients and chains via VLESS+REALITY to the upstream exit. No `freedom` outbound — pure relay.

Steps:

```bash
# 1. add a synced "relay" user (the relay's outbound auths upstream as this user;
#    name typically matches the relay server name)
./scripts/xray-users add <relay-name>

# 2. add the relay entry to servers.json with `relay_upstream` pointing at an
#    existing exit server's name. Pick `xhttp_sni`/`sni` on the same ASN as the
#    relay's IP for REALITY camouflage (e.g. on Selectel use `static.utkonos.ru`
#    which CNAMEs to selcdn.ru on AS49505).
jq '. + [{
  name: "<relay-name>",
  host: "<relay-ip>",
  ssh: "<relay-ssh-alias>",
  public_key: "", private_key: "", short_id: "", xhttp_path: "",
  xhttp_sni: "<ASN-matched-hostname>",
  sni: "<ASN-matched-hostname>",
  relay_upstream: "<exit-server-name>"
}]' servers.json > tmp && mv tmp servers.json

# 3. (optional) mark the upstream exit as upstream-only so clients reach it ONLY
#    via this relay chain — never directly:
jq '(.[] | select(.name=="<exit-server-name>") | .client_render) = false' \
   servers.json > tmp && mv tmp servers.json

# 4. deploy — picks up relay mode automatically from the `relay_upstream` field
NAME=<relay-name> make deploy

# 5. **MANDATORY** for SNI changes: deploy.sh uses `docker compose up -d` which
#    recreates the container but xray's DNS-resolution state for `dest` may
#    persist from the previous run (observed when only IPv6 records are
#    returned for the new SNI but the host has no global v6 route). Force a
#    full restart on the relay AFTER deploy:
ssh <relay-ssh-alias> 'cd /opt/xray && sudo docker compose restart xray'

# 6. re-render client configs. For RU paths, prefer xhttp-only (TCP+vision is
#    more vulnerable to consumer-ISP DPI flow-learning).
./scripts/render-config --proto xhttp
```

Clients can verify the chain works end-to-end with: `curl --socks5 127.0.0.1:<relay-socks-port> https://ifconfig.me` — must return the upstream exit's public IP, not the relay's.

REALITY camouflage check from any vantage:
```bash
openssl s_client -connect <relay-ip>:443 -servername <xhttp_sni> -alpn h2 -tls1_3 </dev/null 2>&1 | grep subject=
# Expect the dest site's real cert (e.g. Lenta/Utkonos OV cert for static.utkonos.ru),
# NOT xray's synthetic cert.
```

## Files

```
.env.example                              - Configuration template
.env                                      - Your config (git-ignored)
users.json                                - UUID → device name map (git-ignored)
servers.json                              - Server list (git-ignored)
docker-compose.yml                        - Server container definition
config/
  server.template.json                    - Server XRay config template (exit mode)
  server-relay.template.json              - Server XRay config template (relay mode: dual-role inbound+chain outbound)
  client/
    sing-box-skeleton.json                - Client sing-box skeleton (urltest, DNS, routes)
    xray-xhttp-skeleton.json              - Client xray-core skeleton (XHTTP SOCKS chain)
    sing-box.template.json                - Legacy single-server sing-box template
    com.xray-xhttp.plist                  - launchd plist for xray-core
    com.sing-box-vpn.plist                - launchd plist for sing-box
scripts/
  deploy.sh       - First-time server deployment
  xray-users      - User management CLI
  render-config   - Render both client configs from templates
  generate-client-config - Generate client package (config + scripts + ZIP)
  validate-config - Validate sing-box config
  vpn-start       - Start VPN (launchd services)
  vpn-stop        - Stop VPN and cleanup
  vpn-install     - Install client package on target Mac
  verify.sh       - Server health check
  update.sh       - Update XRay version
  backup.sh       - Backup config
```

## Environment Variables

Required in `.env`:
- `SERVER` — VPN server IP
- `PUBLIC_KEY`, `PRIVATE_KEY`, `SHORT_ID` — REALITY parameters
- `SNI` — TCP+vision SNI (e.g. `dl.google.com`)
- `XHTTP_PATH` — random XHTTP URL path (generated by deploy)
- `XHTTP_SNI` — XHTTP REALITY SNI (e.g. `speedtest.gcore.com`)
- `UUID` — client UUID (or auto-read from `users.json`)
- `COMPANY_DOMAIN` — corporate search domain for DNS

Optional:
- `TAILSCALE_AUTH_KEY`, `TAILSCALE_HOSTNAME` — only used by `--with-tailscale` (embedded tsnet on the Mac). For the default (Tailscale-on-exit) architecture, the auth key lives on the VPN exit instead.
- `INTERNAL_DNS_1` — corporate DNS server, used by `--with-corp-dns`

## User Management

```bash
# Add user (outputs XHTTP + TCP+vision share URLs)
./scripts/xray-users add "Mom iPhone"

# Get URLs for existing user
./scripts/xray-users url "Mom iPhone"

# Remove user
./scripts/xray-users remove "Mom iPhone"

# Sync local names with server
./scripts/xray-users sync
```

## Maintenance

```bash
make update    # Update XRay to latest
make backup    # Backup config
make verify    # Check server health
```

## Required Tools

Client: `bash`, `ssh`, `jq`, `sing-box` (brew), `xray` (brew), `envsubst`

Server: deployed automatically via Docker

## Credits

Built with assistance from [Claude Code](https://claude.ai/code) (Anthropic) and [Codex](https://openai.com/codex) (OpenAI).
