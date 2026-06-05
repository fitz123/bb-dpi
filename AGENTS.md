# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

XRay REALITY VPN with auto-failover client infrastructure. Two components:
- **Server (VPN exit)**: XRay VLESS REALITY on Docker — XHTTP (port 443, primary) + TCP+vision (port 8443, fallback). Optionally also a Tailscale node (`tailscale up --accept-routes`) so corp-bound traffic exiting xray is forwarded into the tailnet.
- **Client**: sing-box TUN with urltest auto-failover between xray-core SOCKS (XHTTP) and sing-box native (TCP+vision). Default render produces a thin VLESS client with **no embedded Tailscale** — corp/tailnet access is delegated to the VPN exit. `--with-tailscale` opts back into per-Mac embedded tsnet.

Mostly Bash scripts. The `.pkg` installer flow under `client/pkg-build/`, the Go-based `bb-vpn` control-plane binary under `client/bb-vpn/`, and the SwiftUI menu-bar app under `client/menubar/` (BBVPN.app) add a Makefile-driven build pipeline (`make build-pkg`, `make build-bb-vpn-host`, `make build-bb-vpn-pkg`, `make build-menubar`, `make test-bb-vpn`).

## Documentation hierarchy

1. **README.md** — operator-facing quick reference for using the fleet.
2. **Per-decision ADR files** (`docs/adr-NNN-*.md`) when a single
   choice deserves a dedicated record.
3. **`docs/wiki/`** — LLM-maintained knowledge base covering the
   generalisable DPI-evasion research domain (concepts, observed
   adversary behaviors, diagnostics), organised per Karpathy's
   LLM Wiki pattern. Per-decision detail lives in the ADR files
   above; concept/synthesis pages link to them where relevant.

### Wiki schema (auto-loaded)

@docs/wiki/SCHEMA.md

## Commands

### Server Management
```bash
NAME=my-server SSH_HOST=host SERVER=ip make deploy  # Deploy new server
NAME=my-server make deploy                           # Redeploy existing server
make update              # Update XRay on all servers
make verify              # Health check all servers
make backup              # Backup config and secrets
make list                # List users

# User management (operates on ALL servers)
./scripts/xray-users add "Device Name"
./scripts/xray-users url "Device Name"        # Outputs URLs for all servers
./scripts/xray-users enroll-url "Device Name"          # bb-vpn://enroll URI for the .pkg flow
./scripts/xray-users enroll-url --copy "Device Name"   # also pipes the URI through pbcopy
./scripts/xray-users remove "Device Name"
./scripts/xray-users sync    # Sync local names with server
```

### Client Management
```bash
# Render configs (defaults: no embedded Tailscale, no corp DNS — thin VLESS client)
./scripts/render-config
./scripts/render-config --with-corp-dns                # corp DNS via VLESS exit
./scripts/render-config --with-tailscale               # embedded tsnet, magicdns
./scripts/render-config --with-tailscale --with-corp-dns  # legacy (corp DNS via tsnet)
./scripts/render-config --proto tcp-vision             # skip xray-core, TCP+vision only

# Validate sing-box config
./scripts/validate-config [config.json]

# Start/stop VPN (launchd services, installed to ~/.local/bin/)
vpn-start                              # Use existing rendered configs
vpn-start --with-corp-dns              # Re-render with these flags, then start
vpn-start --proto tcp-vision           # Any render-config flag passes through
vpn-stop                               # Unload launchd services and cleanup

# Generate client package (config + scripts + ZIP)
./scripts/generate-client-config "Device Name"                              # defaults — thin VLESS client
./scripts/generate-client-config "Device Name" --with-corp-dns              # + corp DNS via VLESS exit
./scripts/generate-client-config "Device Name" --with-tailscale --with-corp-dns  # embedded tsnet (legacy)
./scripts/generate-client-config "Device Name" --proto tcp-vision           # TCP+vision only
./scripts/generate-client-config "vless://uuid@host:port?..." [flags...]
# Back-compat: a `tskey-...` token in $2 is auto-promoted to
# --with-tailscale --with-corp-dns (matches the pre-flag-inversion default).

# Install client package on target Mac
./scripts/vpn-install [package-dir]
```

### bb-vpn CLI (Phase 5+, post-.pkg-install)
```bash
# Operator-facing CLI shipped by the .pkg, installed at
# /Library/Application Support/bb-dpi/bin/bb-vpn with a
# system-wide symlink at /usr/local/bin/bb-vpn created by postinstall
# (so `sudo bb-vpn …` and bare `bb-vpn …` both resolve without
# absolute paths — /usr/local/bin is on sudo's secure_path).
# Lifecycle subcommands write a sentinel flag the launchd-driven sync ticks
# respect, so the choice survives reboots until reversed.
sudo bb-vpn start      # Clear manually_stopped flag, kickstart sing-box (+ xray if needed)
sudo bb-vpn stop       # Set manually_stopped flag, bootout sing-box + xray
sudo bb-vpn sync       # Force an immediate sync tick (otherwise launchd fires every 15m)
bb-vpn status          # Print status.json (last_sync, last_error, current_issued_at, …)
bb-vpn enroll <bb-vpn://enroll?uuid=…>  # Submit an enrollment URI (the menubar URL handler shells out to this)
bb-vpn --version       # Print version (ldflags-stamped at build time)
```

### .pkg installer (Phase 4 + 5 + 6)
```bash
make build-bb-vpn-host    # Host-arch bb-vpn binary -> build/bb-vpn (dev/test)
make build-bb-vpn-pkg     # Darwin universal bb-vpn -> build/pkg/bb-vpn (for .pkg)
make build-menubar        # Universal BBVPN.app -> build/menubar/BBVPN.app
make test-bb-vpn          # Run client/bb-vpn Go tests
make build-pkg            # Assemble BB-VPN-<ver>.pkg (incl. BBVPN.app) in client/pkg-build/dist/
```

**Bump the version on every update (required).** `bb-vpn --version` is
stamped from `config/control-plane/package-manifest.json` (`bb_vpn`),
and the Makefile builds the binary *from* that field — so shipping a
code change without bumping it produces a new binary that still reports
the old version, indistinguishable in the field. `bb_vpn` doubles as the
whole-package build identity (`client/pkg-build/build.sh` derives the
`.pkg` filename and `pkgbuild --version` from it), so before
`make build-pkg` for **any**
change to what the `.pkg` ships — `client/bb-vpn` Go sources,
BBVPN.app/menubar, plists, scripts, bundled UI, or the baked-in
`control-plane.json` token — bump `bb_vpn` (semver — patch for a fix,
minor for a compatible feature, major for a breaking bundle-schema
change); set
`sing_box`/`xray` to the dropped-in binary versions.
`client/pkg-build/build.sh`'s coupling check enforces manifest↔binary
*agreement*, not the bump — that
discipline is yours. See [`docs/release.md`](docs/release.md) §2a.

Phase 6 adds ad-hoc codesigning to `build.sh` (`codesign -s - --force`
on the standalone bb-vpn/sing-box/xray Mach-Os, `codesign -s - --force
--deep` on `BBVPN.app`; no Apple Developer license, no notarization —
Gatekeeper still shows "unidentified developer" on first install + first
launch) and a
user-facing install page template at
`client/pkg-build/install-page-template.html`. The operator-facing
host/distribute runbook (build, sign, host on a long-random nginx
location, per-user install page via envsubst, token rotation,
verification) lives in [`docs/release.md`](docs/release.md). Future
operators/agents touching the .pkg flow should read it before
modifying `build.sh` or the install page.

`vpn-start` no longer parses its own flags. Any args after the program name
are forwarded verbatim to `render-config`; xray-need is auto-detected from
the rendered sing-box config (presence of any `xhttp-*` SOCKS outbound).

## Architecture

### Server Side
- XRay with two inbounds: XHTTP on 443, TCP+vision on 8443
- Template in `config/server.template.json` gets variable substitution at deploy time
- XHTTP settings: `mode: auto`, `xPaddingBytes` for DPI resistance
- Docker container with `network_mode: host`, `read_only: true`, `no-new-privileges`

### Client Side (sing-box + xray-core)
- sing-box 1.13+. Default render has **no** embedded Tailscale endpoint; the Mac is a thin VLESS client. `--with-tailscale` opts into tsnet+magicdns on the Mac (per-laptop tailnet identity).
- urltest outbound probes all server transports every 30s with `interrupt_exist_connections`
- xray-core runs as launchd service (`com.xray-xhttp`), provides SOCKS proxies (port 1080+i per server) for XHTTP. Stopped automatically when `--proto tcp-vision` is rendered (no `xhttp-*` outbounds in sing-box config)
- sing-box runs as launchd service (`com.sing-box-vpn`), provides TUN with auto-failover
- Skeletons (static structure) + `servers.json` (server list) → `render-config` generates full configs via jq:
  - `config/client/sing-box-skeleton.json` — urltest, DNS, routes, optional Tailscale (no server outbounds)
  - `config/client/xray-xhttp-skeleton.json` — base structure (no inbounds/outbounds)
- Adding a server = one new object in `servers.json`, re-render. Zero template changes.
- Configs rendered to `~/.config/sing-box/config-auto.json` and `~/.config/xray/config.json`

### `render-config` flags
- `--proto all|tcp-vision|xhttp` — which server transport(s) to render. Default `all`.
- `--with-tailscale` — keep the embedded `tailscale` endpoint, `magicdns`, and tailscale-bound route rules. Off by default.
- `--with-corp-dns` — keep the `company-dns` server (resolved from `${INTERNAL_DNS_1}` in `.env`) and the `*.${COMPANY_DOMAIN}` rule. Off by default.
- When `--with-tailscale` is off, route rules with `outbound: tailscale` are **rewritten** to `outbound: auto` (VLESS chain) rather than stripped — so corp traffic still tunnels via the VPN exit. `company-dns` gets the same `detour: tailscale → auto` rewrite when used with `--with-corp-dns` alone.

### Traffic Routing
Default (no flags):
- Corporate subnets (`10.0.0.0/8`, `172.16.0.0/12`) → `auto` outbound (VLESS → VPN exit, which forwards to tailnet via its own Tailscale install)
- Russian domains/IPs (.ru, geoip-ru) → Direct (bypass VPN)
- Local network (`192.168.x.x`, link-local) → Direct via `ip_is_private`
- Everything else → `auto` outbound (urltest across all servers; XHTTP on 443 via xray-core, or TCP+vision on 8443 direct)

With `--with-tailscale`:
- Tailscale peers (`100.64.0.0/10`) and corporate subnets → `tailscale` outbound (embedded tsnet)
- MagicDNS (`*.ts.net`) resolved by tsnet's `magicdns`
- Other rules unchanged

### VPN exit as Tailscale hop (default architecture)
The default render assumes the **VPN exit** carries the Tailscale identity. Provision once per server:
```bash
# On the VPN exit:
sudo tailscale up --accept-routes --hostname=<vpn-exit-name> --auth-key=tskey-...
# In Tailscale admin UI: tag this node so it has the corp ACL access you need.
```
**`--accept-routes` is required, not optional.** Without it, the exit's tailscaled refuses to install the routes advertised by corporate subnet routers, so the host kernel has no path for those private IPs — they fall through to the default route and never reach the tailnet. xray's `freedom` outbound uses the host kernel's routing table, so the routes have to be present there for the chain to complete. Verify after `up`:
```bash
ip route | grep -E '10\.|100\.6'   # should list corp subnets via tailscale0
```
Also required: `net.ipv4.ip_forward=1` and a default `MASQUERADE` rule (typically already set on a stock VPN host; check with `iptables -t nat -L POSTROUTING -n`).

End-to-end packet path:
```
Mac apps
  → sing-box TUN (172.19.0.1)
  → auto outbound (urltest)
  → VLESS+REALITY :443
  → xray on VPN exit (decapsulates)
  → kernel routing → Tailscale interface
  → tailnet subnet router (e.g. tailscale-01) → corp host
```

### Local State
- `servers.json` — array of server objects (host, ssh, keys, paths). Adding a server = append one object.
- `users.json` — maps UUIDs to friendly device names
- `.env` — shared params only (FINGERPRINT, FLOW, TAILSCALE_*, COMPANY_DOMAIN). No per-server data.

### SSH-Based Operations
All server management happens via SSH. Scripts iterate `servers.json` for multi-server operations.

### Key Scripts
- `scripts/deploy.sh` - Deploy a named server: hardening, Docker, REALITY keys, saves to `servers.json`
- `scripts/xray-users` - User CRUD across all servers, VLESS URL generation, and `bb-vpn://` enrollment URI for the .pkg flow
- `scripts/generate-client-config` - Render client configs, package with scripts + servers.json, create ZIP
- `scripts/render-config` - Builds configs from skeletons + `servers.json` via jq (envsubst for shared vars)
- `scripts/validate-config` - Validate config via `sing-box check`
- `scripts/vpn-install` - Install sing-box + xray (via brew), configs, scripts, launchd plists, Finder shortcuts
- `scripts/vpn-start` - Install/update launchd plists (renders `${HOME}`), start xray-core + sing-box services
- `scripts/vpn-stop` - Unload launchd services, pkill fallback, remove search domain

### Security Layers
1. UFW firewall (ports 22, 443, 8443, 80)
2. SSH key-only authentication
3. Docker: `network_mode: host`, `read_only: true`, `no-new-privileges`
4. REALITY protocol encryption
5. XHTTP traffic fragmentation + random padding

## Required Local Tools

`bash`, `ssh`, `scp`, `jq`, `uuidgen`, `openssl`, `nc`, `envsubst`, `sing-box` (via brew), `xray` (via brew)

## Scripting Guidelines

- **Always use timeouts** - Every network operation, external command, or potentially blocking call must have a timeout to prevent scripts from hanging. Use `timeout <seconds> <command>` for commands, or built-in timeouts for tools like `curl -m`, `nc -w`, etc.

## Config Change Safety

- **NEVER restart apps/services before validating config** - Always test configuration syntax/validity BEFORE restarting. A bad config can break VPN connectivity and lock you out.
- For sing-box: use `sing-box check -c config.json` or `./scripts/validate-config` before restart
- For XRay: use `xray -test -config config.json` before container restart
- General rule: read the generated/merged config file and verify your changes are present and correct

## Files That Should Never Be Committed

`.env`, `users.json`, `servers.json` - contain secrets, user UUIDs, and server private keys
