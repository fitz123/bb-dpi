# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

XRay REALITY VPN with auto-failover client infrastructure. Two components:
- **Server (VPN exit)**: XRay VLESS REALITY on Docker — XHTTP (port 443, primary) + TCP+vision (port 8443, fallback). Optionally also a Tailscale node (`tailscale up --accept-routes`) so corp-bound traffic exiting xray is forwarded into the tailnet.
- **Client**: sing-box TUN with urltest auto-failover between xray-core SOCKS (XHTTP) and sing-box native (TCP+vision). The client is a thin VLESS client with **no embedded Tailscale** — corp/tailnet access is always delegated to the VPN exit.

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
# Render configs (defaults: no corp DNS — thin VLESS client)
./scripts/render-config
./scripts/render-config --with-corp-dns                # corp DNS via VLESS exit
./scripts/render-config --proto tcp-vision             # skip xray-core, TCP+vision only

# Validate sing-box config
./scripts/validate-config [config.json]

# Start/stop VPN (launchd services, installed to ~/.local/bin/)
vpn-start                              # Use existing rendered configs
vpn-start --with-corp-dns              # Re-render with these flags, then start
vpn-start --proto tcp-vision           # Any render-config flag passes through
vpn-stop                               # Unload launchd services and cleanup
BB_DPI_ALLOW_OTHER_VPN=1 vpn-start     # Skip the other-VPN guard (env, not a flag: args go to render-config)

# Pre-flight a candidate config beside the running VPN (dev tool, not packaged)
./scripts/test-config-sandbox --with-corp-dns --proto xhttp   # any render-config flags
./scripts/test-config-sandbox --duration 600                  # + e2e/DNS timeline every 5s
./scripts/test-config-sandbox --interface en0 --keep          # pin the egress IF, keep workdir

# Generate client package (config + scripts + ZIP)
./scripts/generate-client-config "Device Name"                              # defaults — thin VLESS client
./scripts/generate-client-config "Device Name" --with-corp-dns              # + corp DNS via VLESS exit
./scripts/generate-client-config "Device Name" --proto tcp-vision           # TCP+vision only
./scripts/generate-client-config "vless://uuid@host:port?..." [flags...]

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
sudo bb-vpn target test   # Fetch the TEST bundle on every tick (sentinel file, survives reboots)
sudo bb-vpn target prod   # Back to the PROD bundle (the default)
bb-vpn target          # Print active publish target (read-only, no root)
bb-vpn status          # Print status.json (last_sync, last_error, current_issued_at, target, …)
bb-vpn enroll <bb-vpn://enroll?uuid=…>  # Submit an enrollment URI (the menubar URL handler shells out to this)
bb-vpn --version       # Print version (ldflags-stamped at build time)
```

### Control-plane publish (dev machine)
```bash
# Two publish targets at two paths on the same cover endpoints:
# prod = /control/bundle.json (fleet default), test = /control/test/bundle.json
# (fetched only by clients flipped with `sudo bb-vpn target test`).
# Staged rollout: publish test → validate on the test client → promote.
make publish-bundle-test   # Working tree → TEST path (staging)
make publish-bundle-prod   # Working tree → PROD path (whole fleet)
make promote-bundle        # Byte-copy validated test bundle → prod (sha-verified, never re-assembles)
make publish-status        # issued_at/sha per (endpoint, target), both targets
make test-publish-bundle   # Semantic allowlist guard (no secret server fields leak)
# `make publish-bundle` (no target) is a stub that errors with pointers.
# Workflow + nginx wiring: docs/control-plane-bootstrap.md; release flow: docs/release.md.
```

### .pkg installer (Phase 4 + 5 + 6)
```bash
make build-bb-vpn-host    # Host-arch bb-vpn binary -> build/bb-vpn (dev/test)
make build-bb-vpn-pkg     # Darwin universal bb-vpn -> build/pkg/bb-vpn (for .pkg)
make build-menubar        # Universal BBVPN.app -> build/menubar/BBVPN.app
make test-bb-vpn          # Run client/bb-vpn Go tests
make test-scripts         # Shell-script tests (vpn-start other-VPN guard, bb-dpi-logrotate safety)
make build-pkg            # Assemble BB-VPN-<ver>.pkg (incl. BBVPN.app + rule-set seeds) in client/pkg-build/dist/
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

**Other-VPN guard.** Before touching any service, `vpn-start` refuses to
run while another VPN is `(Connected)`/`(Connecting)` in `scutil --nc
list` (or, as a backstop, a non-Tailscale `PacketTunnel` NetworkExtension
process is running): two TUN VPNs fighting over routes/DNS cut the
operator's connectivity. Tailscale (`io.tailscale.*`) coexists → warning
only. Override with `BB_DPI_ALLOW_OTHER_VPN=1`. Detection lives in
`detect_other_vpns` (pure function between `# >>> other-vpn-guard`
markers; `scripts/test-vpn-guard` / `make test-scripts` extracts that
block and runs it against inline `scutil --nc list` / `pgrep` fixtures).

**`test-config-sandbox`** pre-flights a config without disconnecting the
running VPN: renders to a temp workdir, `sing-box check` + `xray run
-test`, then runs both unprivileged with a `mixed` inbound + UDP DNS
listener on 127.0.0.1 (no TUN), xray SOCKS ports shifted +20000, every
upstream dial pinned to the physical interface (sing-box `bind_interface`
/ dhcp `interface`; xray `sockopt.interface`). Reports DNS answers (bogus
ranges flagged, answering server + failed legs from the debug log), each
direct DoH upstream, each XHTTP leg, end-to-end + RU-direct routing, and
log FATAL/ERRORs. Exits non-zero on a critical failure (config check,
e2e, `example.com` DNS, or a `--duration` timeline that ends dead).
**Does not cover the TUN layer** (auto_route, route_exclude_address, OS
DNS hijack, coexistence with other VPNs) — use macold for that.

## Architecture

### Server Side
- XRay with two inbounds: XHTTP on 443, TCP+vision on 8443
- Template in `config/server.template.json` gets variable substitution at deploy time
- XHTTP settings: `mode: auto`, `xPaddingBytes` for DPI resistance
- Docker container with `network_mode: host`, `read_only: true`, `no-new-privileges`

### Client Side (sing-box + xray-core)
- sing-box 1.14+ (DNS `evaluate`/`race` rules; `vpn-install` upgrades older brew installs). No embedded Tailscale endpoint; the Mac is a thin VLESS client and corp/tailnet traffic goes via the VPN exit.
- urltest outbound probes all server transports every 30s with `interrupt_exist_connections`
- xray-core runs as launchd service (`com.xray-xhttp`), provides SOCKS proxies (port 1080+i per server) for XHTTP. Stopped automatically when `--proto tcp-vision` is rendered (no `xhttp-*` outbounds in sing-box config)
- sing-box runs as launchd service (`com.sing-box-vpn`), provides TUN with auto-failover
- Skeletons (static structure) + `servers.json` (server list) → `render-config` generates full configs via jq:
  - `config/client/sing-box-skeleton.json` — urltest, DNS, routes (no server outbounds)
  - `config/client/xray-xhttp-skeleton.json` — base structure (no inbounds/outbounds)
- Adding a server = one new object in `servers.json`, re-render. Zero template changes.
- Configs rendered to `~/.config/sing-box/config-auto.json` and `~/.config/xray/config.json`
- Remote rule-sets (`geosite-*`, `geoip-*`) carry `initial_path: rule-sets/{tag}.srs`, resolved against sing-box's working directory (both `com.sing-box-vpn` plists set `WorkingDirectory`: `~/.config/sing-box` / `/Library/Application Support/bb-dpi`). With no cached copy, sing-box loads that seed instead of downloading, so first start does not depend on GitHub (a failed initial download with no seed is FATAL). Seeds come from `scripts/fetch-rule-sets` — `build.sh` stages them into the .pkg, `generate-client-config` packages them, `vpn-install` copies them to `~/.config/sing-box/rule-sets/`. Never committed (`.gitignore`).
- `com.bb-dpi.logrotate` LaunchDaemon runs `bb-dpi-logrotate` (copytruncate log rotation, see Logs below). Bash flow: `config/client/com.bb-dpi.logrotate.plist` → loaded by `vpn-start`, left loaded by `vpn-stop`. .pkg flow: `client/plists/com.bb-dpi.logrotate.plist` → bootstrapped by postinstall, removed by `bb-vpn-uninstall`.

### Logs
- **Levels**: sing-box `log.level: "warn"`, xray `log.loglevel: "warning"` (skeletons). At `info` sing-box logs every connection — one client's `vpn.log` reached 10.5 GB.
- **Where**: bash flow `~/.config/sing-box/vpn.log`, `~/.config/xray/xray.log` (+ `~/.config/sing-box/logrotate.log`); .pkg flow `/Library/Logs/bb-dpi/{sing-box,xray,bb-vpn-sync,bb-vpn-menubar,logrotate}.log`.
- **Rotation**: `scripts/bb-dpi-logrotate` (shipped as `bin/bb-dpi-logrotate` in the .pkg; in the bash flow `vpn-start` installs a root-owned copy at `/usr/local/libexec/bb-dpi-logrotate`, since the daemon runs as root and must not execute a user-writable file), every 30 min + at load. Any log over 20 MB (`--max-bytes` / `BB_DPI_LOGROTATE_MAX_BYTES`) is gzipped to `<log>.1.gz` (one generation, replaced each time) and the live file truncated in place. **copytruncate, never rename**: the daemons write to the stdout fd launchd opened (with `O_APPEND`) and never reopen it, so newsyslog-style rename would keep them writing into the renamed file; truncation is safe because of `O_APPEND` (no sparse regrow). It runs as root over user/admin-writable log dirs, so it is a perl script that resolves each log path once and works on file descriptors only (`O_NOFOLLOW` open, regular file with `st_nlink == 1`, `O_EXCL|O_NOFOLLOW` temp archive + `rename(2)`, `ftruncate` on the open handle only after the archive is in place) — see its header before changing it.
- **Debugging at higher verbosity** (temporary, do not commit): bash flow — edit `log.level` to `info`/`debug` in `~/.config/sing-box/config-auto.json` (or `loglevel` in `~/.config/xray/config.json`), validate (`sing-box check` / `xray -test`), then `vpn-start` (no args, so it does not re-render). Revert by re-running `vpn-start` with render flags. .pkg flow — the level comes from the published bundle's skeletons, so a local edit to `/Library/Application Support/bb-dpi/configs/*.json` may be overwritten by a later sync tick; for a one-off, edit + validate + `sudo launchctl kickstart -k system/com.sing-box-vpn`, or (durable) publish a TEST bundle with a raised level and flip the client with `sudo bb-vpn target test`. Only one `.1.gz` generation is kept, so a long session at `debug` drops older lines at each rotation — copy the log out as you go.

### `render-config` flags
- `--proto all|tcp-vision|xhttp` — which server transport(s) to render. Default `all`.
- `--with-corp-dns` — keep the `company-dns` server (resolved from `${INTERNAL_DNS_1}` in `.env`, queried via the `auto` VLESS chain) and the `*.${COMPANY_DOMAIN}` rule. Off by default.

### Traffic Routing
- Corporate subnets (`10.0.0.0/8`, `172.16.0.0/12`) → `auto` outbound (VLESS → VPN exit, which forwards to tailnet via its own Tailscale install)
- Local network (`192.168.x.x`, link-local) → Direct via `ip_is_private`
- Known-Russian services by name — Yandex (`yandex.ru/.net/.com`, `ya.ru`, `yastatic.net`) and VK (`vk.com`, `vk.ru`, `userapi.com`, `vkuser.net`) — plus Alibaba Cloud (`.aliyuncs.com`, `.aliyun.com`, `.alibabacloud.com`) → Direct regardless of IP
- IPs of countries not blocked by / friendly to RU → Direct (`geoip-*`: RU, BY, KZ, UZ, KG, TJ, TM, AM, AZ, GE, TR, CN, HK, MO, MN, AE, IR, IN, VN, TH, ID, LA, MM)
- Other RU/CN **domains are NOT routed direct by name** (no `.ru`/geosite rule) — only by where they are hosted (the geoip rule above). A `.ru` site on Cloudflare or other foreign hosting goes through the tunnel: RU DPI throttles direct connections to Cloudflare (transfers freeze after ~16–20 KB), so sending such a site direct breaks it. RU/CN names are still *resolved* via AdGuard (see DNS).
- Everything else → `auto` outbound (urltest across all servers; XHTTP on 443 via xray-core, or TCP+vision on 8443 direct)

### DNS (sing-box ≥ 1.14 required)
All **public** resolvers (AdGuard, Google and Cloudflare DoH, `network-dns`) are dialed **direct** — public DNS never rides the VLESS chain (a relay flap must not hang resolution). The one exception is `company-dns`, present only with `--with-corp-dns`: it has `detour: auto`, so `*.${COMPANY_DOMAIN}` lookups **do** go through the tunnel and can stall during a relay flap — check the chain first when corp names hang but public names resolve. Rules, in order:
1. `*.${COMPANY_DOMAIN}` → `company-dns` (via `auto`; only with `--with-corp-dns`).
2. RU/CN domains → `evaluate` AdGuard DoH; answer returned only if NOERROR and **not bogus**.
3. Everything else (and RU/CN fall-through) → Google DoH (8.8.8.8) and Cloudflare DoH (1.1.1.1) queried in parallel (`race`); first healthy, non-bogus answer wins. NXDOMAIN is returned as-is.
4. Last resort → `network-dns` (`type: dhcp`, the network's own resolver, plaintext) — same bogus filter: a clean answer or NXDOMAIN is returned, anything else (loopback/private/sinkhole answers, failures) is **REFUSED** rather than passed through.

"Bogus" = any A/AAAA in private/loopback/unspecified/CGNAT/benchmark/reserved ranges (`0.0.0.0`, `127.x`, `10.x`, `100.64/10`, `198.18/15`, `::`, `::1`, …): ad-block sinkholes, ISP stub pages, hijacking Wi-Fi gear. Such answers are discarded and resolution falls through. Some networks hijack 1.1.1.1 or 8.8.8.8 (e.g. Cisco WLC answering on 1.1.1.1:443 with a self-signed cert) — the race makes that a non-event. DNS cache: sing-box ≥ 1.14 caches per server and honors upstream TTL, failures are never cached; do **not** set `store_dns`, `store_rdrc`, `disable_expire`, `optimistic` or `independent_cache`.

### VPN exit as Tailscale hop
The client relies on the **VPN exit** carrying the Tailscale identity. Provision once per server:
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
- `.env` — shared params only (FINGERPRINT, FLOW, COMPANY_DOMAIN, INTERNAL_DNS_1). No per-server data.

### SSH-Based Operations
All server management happens via SSH. Scripts iterate `servers.json` for multi-server operations.

### Key Scripts
- `scripts/deploy.sh` - Deploy a named server: hardening, Docker, REALITY keys, saves to `servers.json`
- `scripts/xray-users` - User CRUD across all servers, VLESS URL generation, and `bb-vpn://` enrollment URI for the .pkg flow
- `scripts/generate-client-config` - Render client configs, package with scripts + servers.json, create ZIP
- `scripts/render-config` - Builds configs from skeletons + `servers.json` via jq (envsubst for shared vars)
- `scripts/validate-config` - Validate config via `sing-box check`
- `scripts/vpn-install` - Install sing-box + xray (via brew), configs, scripts, launchd plists, Finder shortcuts
- `scripts/vpn-start` - Refuse if another VPN is connected, install/update launchd plists (renders `${HOME}`), start xray-core + sing-box services
- `scripts/test-config-sandbox` - Dev pre-flight of a candidate config on 127.0.0.1 beside the live VPN (no TUN, no root)
- `scripts/vpn-stop` - Unload launchd services, pkill fallback, remove search domain
- `scripts/bb-dpi-logrotate` - Size-based copytruncate rotation of client logs (run by the `com.bb-dpi.logrotate` LaunchDaemon in both client flows)
- `scripts/fetch-rule-sets` - Download every remote rule-set in the sing-box skeleton to `<out-dir>/<tag>.srs` (seeds for `initial_path`)
- `scripts/test-vpn-guard` - Fixture tests for `vpn-start`'s `detect_other_vpns` (`make test-scripts`)

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
