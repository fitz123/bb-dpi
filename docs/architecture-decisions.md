# Architecture Decisions

A single-document register of the architectural and configuration decisions made in
this project. This is intentionally broader than a per-decision ADR — it captures
the *why* behind every load-bearing choice, organised by domain.

Each entry follows the same shape:

- **Decision** — what was chosen
- **Why** — the context / problem it solves
- **Trade-offs** — what's lost, what's gained
- **Citations** — code path, PR number, or supporting doc

Per-decision ADRs (`docs/adr-NNN-*.md`) remain the canonical record when a single
choice deserves a separate file. ADR-001 (no `tcp_multi_path` on Darwin) is the
only such file today; everything else lives here.

## Threat model & fleet

The adversary is **active in-path DPI on consumer ISPs** in the targeted egress
region — capable of:
- Active TLS probing (REALITY anti-probe is the primary defence)
- SNI/IP cross-correlation (drives ASN-match SNI strategy)
- Payload-aware drops keyed on (SNI, destination ASN) tuples
- Time-windowed flow-learning from sustained probe-failure bursts
- DNS↔TLS metadata joins on plaintext UDP DNS

The fleet is small and operator-driven:
- Two server roles: **exit** (direct internet egress + corporate tailnet hop) and
  **relay** (chains into an upstream exit via VLESS+REALITY).
- Two client classes: **test client** (verifies every change first) and **primary
  client(s)** (daily-driver Macs).
- ~3 servers total, ~2 humans, no multi-tenant or commercial-VPN concerns.

---

## 1. Protocol & transport

### 1.1 VLESS + REALITY as the base tunnel protocol
- **Decision**: REALITY is the TLS-on-the-wire identity for every inbound. All
  camouflage is built around it.
- **Why**: REALITY uniquely forwards active probes to a real TLS site (the
  `dest` field) so probing the IP returns a real cert chain — the server is
  indistinguishable from a TLS reverse proxy without the REALITY key. WireGuard
  and OpenVPN have distinctive handshake patterns; shadowsocks-2022 is
  detectable by entropy analysis.
- **Trade-offs**: REALITY requires careful per-server SNI/dest tuning (§4). A
  bad `dest` breaks the camouflage. Latency-delta active-probe attack remains
  (§7.1).
- **Citations**: `config/server.template.json:24-30,51-57`; `config/server-relay.template.json:25-30`.

### 1.2 Dual transport: XHTTP on 443 + TCP+vision on 8443
- **Decision**: Every exit and relay server runs two REALITY inbounds in one
  xray process — XHTTP on 443 (primary) and TCP+vision (`xtls-rprx-vision`) on
  8443 (fallback).
- **Why**: Port 443 + HTTP/2-like fragmentation is the most stable transport.
  TCP+vision is retained as a fallback for transport-specific outages — urltest
  can route around a transport-level failure. The two transports have different
  observable signatures.
- **Trade-offs**: Two ports → double the firewall surface and two REALITY
  identities (different SNI per transport allowed). UDP-based transports
  (Hysteria2, QUIC) were tried in PR #2 and abandoned — Hysteria2 streams
  timed out at scale.
- **Citations**: `config/server.template.json:5-63`; `config/server-relay.template.json:6-65`; PR #2 (closed).

### 1.3 XHTTP `mode: auto`
- **Decision**: `xhttpSettings.mode: "auto"` on every XHTTP inbound.
- **Why**: Auto adapts framing per-request between `packet-up` (POST-per-chunk,
  more HTTP-like) and `stream-up` (single-POST streaming). Picks the more
  HTTP-shaped transport when middleboxes inspect chunk boundaries.
- **Citations**: `config/server.template.json:21`; `config/server-relay.template.json:22`.

### 1.4 `xPaddingBytes: "100-1000"` random per-request padding
- **Decision**: Per-request random padding in the [100,1000] byte range.
- **Why**: Traffic-shape analysis on REALITY-tunneled HTTP can profile
  request-size distribution. Real CDN traffic has bursty variable-length
  requests; an unpadded tunnel shows tight clusters at MSS multiples. Random
  padding widens the histogram.
- **Trade-offs**: 5-15% bandwidth overhead.
- **Citations**: `config/server.template.json:22`; `config/server-relay.template.json:23`.

### 1.5 `tcp_multi_path` removed (ADR-001)
- **Decision**: Never set `tcp_multi_path: true` on client VLESS outbounds.
- **Why**: On Darwin 24.6 ARM64, MPTCP caused `dial tcp ...: invalid argument`
  on data transfer while urltest probes still passed → silent connectivity
  loss. Also: MPTCP TCP options are a fingerprint divergence from native
  Chrome/Safari on macOS.
- **Citations**: `docs/adr-001-remove-tcp-multi-path.md`.

### 1.6 `tcp_fast_open: false` default on client TCP+vision outbounds
- **Decision**: TFO is off by default on `tcp-*` outbounds.
- **Why**: Real Chrome / Safari / Firefox on macOS do NOT enable TFO by
  default. A TFO `cookieReq` SYN while claiming uTLS-chrome at the TLS layer
  is fingerprint-incoherent — macOS outbound TFO to non-Apple IPs is dominated
  by VPN/proxy clients. Memory `feedback_twosided_tcpdump_dead_tunnel.md`
  already named TFO `cookiereq` as a common trigger for SYN-ACK retransmit
  failures.
- **Trade-offs**: ~30-100ms slower probe init — invisible against the
  multi-RTT REALITY+vision handshake.
- **Citations**: `scripts/render-config:178`; PR #11; `docs/adr-001-remove-tcp-multi-path.md` Update section.

### 1.7 No PROXY protocol (`xver: 0`)
- **Decision**: REALITY `xver` is never set; xray defaults to 0.
- **Why**: With `xver: 2` xray would send a PROXY-protocol header to `dest`
  containing the client's source IP. The `dest` site is third-party (CDN,
  cloud object store) — leaking client IPs to it is wrong and is itself a
  fingerprintable behavior.
- **Citations**: `config/server.template.json` (no `xver` field); deploy
  validation step "xver paranoia check" in plan
  `docs/plans/20260514-asn-match-sni-and-xhttp-only-chain.md`.

---

## 2. Server architecture

### 2.1 Two server roles: exit and relay
- **Decision**: One xray image, two templates. Role is determined by the
  optional `relay_upstream` field in `servers.json` — absent = exit, present =
  relay chained to that upstream's name.
- **Why**: Same binary, same hardening, same deploy script. Schema-driven role
  selection (one field) keeps a single source of truth.
- **Alternatives considered**: separate `upstreams.json` for relay-only
  servers (rejected — would break `make deploy NAME=aws-st` uniformity);
  hardcoding the upstream's host/keys into the relay's own entry (rejected —
  duplicates state).
- **Citations**: `scripts/deploy.sh:62, 70-84, 240-298`; `config/server-relay.template.json`; PR #9.

### 2.2 Relay user UUID = relay's server name
- **Decision**: A relay's outbound authenticates upstream as a synced user
  whose `users.json` value equals the relay's `name` (e.g., server `gigi` uses
  user `gigi`).
- **Why**: Multiplexes all client traffic through one upstream UUID, keeps the
  upstream's `clients` array small, lets the operator identify chain traffic
  on the upstream by user-email.
- **Trade-offs**: Each relay needs one extra user UUID synced to every server
  in the fleet (handled by `xray-users` which iterates all servers regardless
  of `client_render`).
- **Citations**: `scripts/deploy.sh:86-101`.

### 2.3 Pure-relay (no `freedom` outbound on relays)
- **Decision**: Relay template has only `to-upstream-xhttp`,
  `to-upstream-tcp`, and `block` outbounds — no `direct`/freedom.
- **Why**: Defence in depth — if routing rules ever fail open, traffic gets
  blackholed instead of leaking from the relay's own IP.
- **Trade-offs**: No graceful fallback if upstream becomes unreachable; the
  relay simply stops serving. Acceptable.
- **Citations**: `config/server-relay.template.json:65-133` (outbounds),
  `:136-149` (routing).

### 2.4 Inbound-tag → outbound-tag routing on relays
- **Decision**: Relay's `routing.rules` statically pin `xhttp-in` →
  `to-upstream-xhttp` and `tcp-in` → `to-upstream-tcp`. No
  sniffing-driven routing.
- **Why**: Forwarder semantics. Client's transport choice must propagate
  end-to-end; the relay does not inspect or override.
- **Citations**: `config/server-relay.template.json:136-149`.

### 2.5 Sniffing posture is role-asymmetric
- **Decision**: Exit template has `sniffing.enabled: true` (both inbounds);
  relay template has `sniffing.enabled: false` (PR #11).
- **Why**: On a pure relay, static inbound-tag routing means the sniffer
  buys nothing while writing every client tunnel's inner SNI into xray
  session metadata. The relay is the most-exposed host (adversarial
  jurisdiction); cutting that metadata exposure is net-positive. The exit is
  in a lower-risk jurisdiction and may want sniffing-based routing in the
  future (currently unused).
- **Open trade-off**: Exit sniffing was explicitly out of scope of PR #11 but
  the same metadata-leak concern applies to a lesser degree.
- **Citations**: `config/server.template.json:32-35,59-62`;
  `config/server-relay.template.json:33-35,61-63`; PR #11.

### 2.6 `servers.json` schema
- **Decision**: Each server entry has: `name`, `host`, `ssh`, `public_key`,
  `private_key`, `short_id`, `xhttp_path`, `xhttp_sni`, `sni`. Optional:
  `relay_upstream` (string), `client_render` (bool, default true).
  Gitignored — contains REALITY private keys.
- **Why**: Single source of truth for fleet state. `name` is the operational
  identifier; `ssh` is the SSH host/alias. Two SNI fields allow per-transport
  tuning when needed.
- **Citations**: `scripts/deploy.sh:48-66, 104-138`.

### 2.7 `save_server()` deep-merges prior + entry (preserves operator fields)
- **Decision**: On every redeploy, `save_server` reads the prior entry,
  builds a new entry from deploy params, then does `$prior + $entry` (deep
  merge). Operator-set fields the deploy doesn't own (`client_render`, future
  per-server knobs) survive the merge.
- **Why**: Earlier versions enumerated fields explicitly and silently
  stripped `client_render: false` on redeploy — defeating the entire
  feature. PR #10 introduced an explicit `has("client_render")` guard
  preserving the flag; PR #11 generalised that into `$prior + $entry` so
  any operator-set field survives without code changes. (The `// null`
  shorthand was considered and rejected — jq treats `false` as null for
  the `//` operator, which would have silently dropped explicit
  `client_render: false` values.)
- **Citations**: `scripts/deploy.sh:104-138`; PR #11.

### 2.8 `client_render: false` for upstream-only servers
- **Decision**: Optional boolean field on `servers.json` entries; default
  true. Servers with `client_render: false` are filtered out of: sing-box
  outbounds, urltest pool, `route_exclude_address`, xray-core SOCKS
  inbounds/outbounds, and the bundled `servers.json` in the client package.
- **Why**: Upstream-only relay targets must not appear in client urltest
  pools — direct probes from clients trigger DPI flow-learning (see §5).
  Keeps a single source of truth in `servers.json` while letting render
  filter aggressively.
- **Trade-offs**: `xray-users` deliberately does NOT honor the flag — relay
  chains need the upstream's synced UUID list for outbound auth.
- **Citations**: `scripts/render-config:106-110, 155, 266`; PR #10; README §
  Architecture.

### 2.9 Filter uses `!= false`, not `// true` (jq footgun)
- **Decision**: Filter expression is `select(.client_render != false)`. NOT
  `select(.client_render // true)`.
- **Why**: jq's `//` operator triggers on null AND false, so `false // true`
  returns `true` — would silently keep `client_render: false` entries.
- **Citations**: `scripts/render-config:155`; PR #10 commit message.

### 2.10 Filter runs BEFORE `to_entries[]`
- **Decision**: Apply `map(select(.client_render != false))` before any
  `to_entries[]` or iteration.
- **Why**: SOCKS port assignment is `1080 + i` where `i` is the index in the
  visible-server list. Filtering after `to_entries[]` would leave gaps;
  filtering before keeps indices sequential, matching xray-core and sing-box.
- **Citations**: `scripts/render-config:155-158,266`.

### 2.11 Single source of truth for SNI per server
- **Decision**: REALITY `serverNames` array contains exactly one entry, equal
  to the `dest` hostname (without port). Never a wildcard.
- **Why**: REALITY forwards probes to `dest`; if the inbound accepts a SNI
  the server cannot also forward, the camouflage is broken. Codex consultation
  confirmed: literal SNI only.
- **Citations**: `config/server.template.json:26-28`; plan
  `docs/plans/20260514-asn-match-sni-and-xhttp-only-chain.md`.

### 2.12 Per-server keys (REALITY x25519, short_id, xhttp_path) generated once
- **Decision**: Keys generated at first deploy via `xray x25519` inside the
  container; `short_id` via `openssl rand -hex 8`; `xhttp_path` via the same.
  All persisted in `servers.json`, never rotated unless server is redeployed
  fresh.
- **Why**: No automated rotation infrastructure; rotation requires coordinated
  client+server config swap (multi-client). Accepted lifecycle gap.
- **Trade-offs**: A single key compromise affects every client of that
  server. Mitigated by §3 hardening; revisit if a REALITY auth-bypass CVE
  ever lands.
- **Citations**: `scripts/deploy.sh:231-263`.

---

## 3. Server hardening

### 3.1 UFW: `deny incoming` default, allow 22 / 80 / 443 / 8443
- **Decision**: Four ports allowed; everything else dropped.
- **Why**: Minimal internet-exposed surface. 22 for management (key-only),
  443/8443 for REALITY, 80 reserved (ACME-style validation if ever needed).
- **Citations**: `scripts/deploy.sh:155-162`.

### 3.2 SSH password auth disabled via a high-priority drop-in
- **Decision**: `deploy.sh` writes
  `/etc/ssh/sshd_config.d/00-bb-dpi-hardening.conf` disabling
  `PasswordAuthentication`, `KbdInteractiveAuthentication`,
  `ChallengeResponseAuthentication`. Validates with `sshd -t`, reloads (not
  restarts — preserves open sessions), then hard-fails the deploy if `sshd
  -T` doesn't confirm all three are `no`.
- **Why**: Cloud images often ship `/etc/ssh/sshd_config.d/50-cloud-init.conf`
  with `PasswordAuthentication yes` that silently overrides a main-file `no`.
  sshd uses first-occurrence-wins after `Include` near the top of the main
  file, so a `00-`-prefixed drop-in loads before the cloud-init one and
  wins.
- **Trade-offs**: Two operational concerns folded in (validate then reload,
  effective-state postcondition) after Copilot review on PR #11.
- **Citations**: `scripts/deploy.sh:163-194`; PR #11.

### 3.3 SSH reload, not restart
- **Decision**: After config change, `systemctl reload ssh` (Debian) or
  `reload sshd` (RHEL family). Never `restart`.
- **Why**: Reload preserves existing SSH sessions — critical because a bad
  config + restart can lock the operator out. The validate-then-reload pattern
  uses separate `if !sshd -t; then exit; fi` to ensure a failed validation
  blocks the reload (operator-precedence trap caught by Copilot on PR #11).
- **Citations**: `scripts/deploy.sh:175-194`.

### 3.4 Docker hardening: host-network + read-only + no-new-privileges
- **Decision**: `network_mode: host`, `read_only: true`,
  `security_opt: no-new-privileges:true`, `user: root` (required for
  binding 443/8443 with host networking).
- **Why**: REALITY needs raw socket access on privileged ports.
  `network_mode: host` removes a NAT layer and simplifies multi-port REALITY.
  `read_only` + `no-new-privileges` defang most container escape vectors.
- **Trade-offs**: An xray RCE = root on the host. Mitigated by xray's track
  record + read-only fs; accepted for simplicity.
- **Citations**: `docker-compose.yml:5-13`.

### 3.5 Unattended upgrades enabled on every deploy
- **Decision**: `deploy.sh` enables Debian's `unattended-upgrades` for
  security patches.
- **Why**: Patches OS-level CVEs without operator intervention. Especially
  important for sshd and kernel.
- **Citations**: `scripts/deploy.sh:196-198`.

### 3.6 Log rotation: 10MB × 3 files
- **Decision**: docker-compose json-file driver with 10MB/file and 3 files
  retained.
- **Why**: Caps disk usage on verbose xray logs; long-uptime containers
  otherwise fill disks.
- **Citations**: `docker-compose.yml:14-18`.

### 3.7 Docker image pinned to `:latest` with `compose pull` on every deploy
- **Decision**: `image: ghcr.io/xtls/xray-core:latest`; `make deploy` and
  `make update` always pull.
- **Why**: Auto-updates to the newest xray on every redeploy. Caught at
  least one regression — newer xray's `x25519` output format change broke the
  `generate_keys` regex (fixed in `c4f9a30`).
- **Trade-offs**: Non-reproducible builds. Acceptable for a small fleet.
- **Citations**: `docker-compose.yml:3`; `scripts/deploy.sh:306`.

### 3.8 Explicit `docker compose restart xray` after `up -d`
- **Decision**: `start_container()` runs `docker compose restart xray` AFTER
  `docker compose pull && up -d`.
- **Why**: `up -d` is idempotent — when image and compose file are
  unchanged, it leaves the running container alone. But the bind-mounted
  `/opt/xray/config.json` is what xray reads at process start, so a config-only
  edit silently doesn't take effect without an explicit restart. Observed: an
  SNI swap stuck on the prior cert chain until manual restart.
- **Trade-offs**: Adds ~5s of unavailability per deploy. Acceptable.
- **Citations**: `scripts/deploy.sh:328-340`; PR #11.

### 3.9 Tailscale on the exit, NOT per-client (default architecture)
- **Decision**: VPN exit servers run `tailscale up --accept-routes`. Client
  Macs are thin VLESS clients by default; corporate routes flow exit →
  tailnet via xray's freedom outbound + host kernel routing.
- **Why**: Single Tailscale identity per region beats per-client identity
  proliferation; corporate ACLs apply at the exit's tag; no DERP-region
  edge cases on the client; no MTU-over-DERP issues.
- **Trade-offs**: Loss of per-client visibility in the tailnet admin UI.
  Embedded `tsnet` remains available via `--with-tailscale` render flag.
- **Citations**: `README.md` § Architecture; PR #8.

### 3.10 `tailscale up --accept-routes` is REQUIRED on exits
- **Decision**: `--accept-routes` is mandatory on every exit, not optional.
- **Why**: Without it, tailscaled refuses to install corporate subnet routes
  locally, so xray's freedom outbound has no kernel path for corp IPs —
  they fall through to the default route and never reach the tailnet.
  Verification: `ip route` must show corp subnets via `tailscale0`.
- **Citations**: `README.md` § Tailscale-on-VPN-exit; AGENTS.md.

### 3.11 Tailscale install is a manual one-time provisioning step
- **Decision**: `deploy.sh` does NOT install or configure Tailscale.
  Operator runs the install script + `tailscale up` once per exit, then
  tags the node in the admin UI for ACL scope.
- **Why**: Auth keys and ACL tagging are out-of-band, operator-mediated
  decisions.
- **Citations**: `README.md` § Tailscale-on-VPN-exit.

---

## 4. SNI selection

### 4.1 ASN-match heuristic
- **Decision**: The hostname used in REALITY `dest` and `serverNames` must
  resolve to an IP on the same ASN as the VPN server's own IP.
- **Why**: REALITY forwards active probes raw to `dest`. A passive observer
  correlating "this IP claims to host site X" against "site X's actual
  hosting ASN" can flag mismatches. ASN-match removes that signal.
- **Trade-offs**: Limits SNI choices to "real sites hosted on the same ASN as
  the server". Not a perfect defence — per-prefix or per-cluster heuristics
  defeat it; latency-delta probes ignore it entirely (§7.1).
- **Citations**: `README.md` § Architecture; PR #10; plan
  `docs/plans/20260514-asn-match-sni-and-xhttp-only-chain.md`.

### 4.2 Per-server SNI choice in `servers.json`
- **Decision**: Each server has `xhttp_sni` (port 443) and `sni` (port 8443)
  in `servers.json`; templates substitute via `<XHTTP_SNI>` / `<SNI>`
  placeholders. Two SNI fields per server allows different SNIs per
  transport when needed.
- **Why**: Per-server hosting providers have different ASNs; the camouflage
  hostname must match each. The two transports can use different SNIs to
  blend with different traffic shapes if a single hostname proves
  observable on only one transport.
- **Citations**: `scripts/deploy.sh:60-61, 115-116`; templates' placeholders.

### 4.3 SNI choice strategy per server role
- **Decision**: Three patterns, one per role:
  - **Direct exit**: prefer ASN-matched SNI; accept a known-good
    non-ASN-matched SNI if a retried-match attempt caused unpredictable
    wedges.
  - **Upstream-only exit**: ASN-matched, region-plausible
    object-store-style hostname.
  - **Relay**: ASN-matched, CDN-style hostname that CNAMEs into the same
    ASN, validated to accept vanilla TLS ClientHellos without WAF
    interference.
- **Why**: ASN-match is the rule; per-server empirical results override
  when a strict match causes outages. Validate the WAF-passes-vanilla
  property of any candidate `dest` (Codex confirmed this is needed).
- **Concrete values**: kept out of the repo per the project's PII rule.
  Live values are in gitignored `servers.json` and project memory.
- **Citations**: plan
  `docs/plans/20260514-asn-match-sni-and-xhttp-only-chain.md`.

### 4.4 `dest` SNI must accept vanilla ClientHellos
- **Decision**: When selecting a `dest`, verify the target accepts unmodified
  TLS ClientHellos without WAF interference.
- **Why**: REALITY's MirrorConn forwards the probe's bytes raw to `dest`. If
  the dest's WAF rejects vanilla traffic (e.g., openresty fingerprinting),
  active probers see WAF errors that don't match the camouflage. Confirmed
  during plan `20260514-asn-match-sni-and-xhttp-only-chain.md` (Codex
  consultation): the chosen dest hostname must not be behind a strict WAF.
- **Citations**: same plan, "Alternatives considered" section.

---

## 5. Client architecture

### 5.1 Two-process client: sing-box + xray-core
- **Decision**: sing-box owns TUN, urltest, DNS, and routing. xray-core
  runs in parallel providing per-server SOCKS proxies for the XHTTP
  transport (one SOCKS inbound on `127.0.0.1:1080+i` per visible server).
- **Why**: sing-box does not natively implement XHTTP. xray-core does.
  Running both lets the client use the best of each — sing-box's routing
  and urltest + xray-core's XHTTP transport.
- **Trade-offs**: Two services to monitor/restart; doubled IPC overhead.
- **Citations**: `scripts/render-config:142-205, 257-326`; AGENTS.md
  Architecture/Client Side.

### 5.2 SOCKS port = `1080 + visible_index`
- **Decision**: Each visible server's xray-core SOCKS inbound is on port
  `1080 + i` where `i` is its index in the `client_render != false` filtered
  list. sing-box's `xhttp-<name>` outbound `server_port` matches.
- **Why**: Stable, predictable mapping with no explicit cross-references.
  The filter must run before `to_entries[]` (§2.10) so indices stay
  sequential across the visible subset.
- **Trade-offs**: Adding or removing a server reshuffles ports; both
  rendered configs must be re-rendered together.
- **Citations**: `scripts/render-config:161-167, 270-278`; PR #10.

### 5.3 launchd LaunchDaemons (system-scope, root)
- **Decision**: Both services run as system `LaunchDaemon`s:
  `com.sing-box-vpn`, `com.xray-xhttp`. `RunAtLoad=true`, `KeepAlive=true`,
  logs in `~/.config/<svc>/`.
- **Why**: TUN requires root, and PID-1-supervised processes survive Mac
  sleep cycles better than user-space LaunchAgents.
- **Trade-offs**: Requires sudo on every `vpn-start`/`vpn-stop`. `${HOME}` is
  rendered into the plist at install time and re-rendered on every
  `vpn-start` to handle HOME moves.
- **Citations**: `config/client/com.{sing-box-vpn,xray-xhttp}.plist`;
  `scripts/vpn-install`; `scripts/vpn-start`.

### 5.4 `vpn-start` forwards all args to `render-config`
- **Decision**: `vpn-start` does NOT parse its own flags. Args after the
  program name are forwarded verbatim to `render-config`; the script
  auto-detects whether xray is needed by inspecting the rendered sing-box
  config for any `xhttp-*` outbound.
- **Why**: Single source of truth for flags. Any new `render-config` flag is
  available via `vpn-start` for free.
- **Trade-offs**: `vpn-start` is dev-side only on installed clients — see
  §6.5.
- **Citations**: `scripts/vpn-start`; PR #8.

### 5.5 TUN: `172.19.0.1/30`, MTU 1280, gvisor stack
- **Decision**: Small private subnet outside common LAN ranges (192.168 /
  10) and the Tailscale CGNAT (100.64). MTU 1280 (IPv6 minimum, conservative
  for any path). gvisor userspace TCP/UDP stack.
- **Why**: Avoids collisions with home LANs and Tailscale. MTU 1280 survives
  any encapsulation overhead. gvisor sidesteps kernel TCP quirks (see ADR-001).
- **Trade-offs**: 1280 MTU is a packet-size fingerprint (encapsulated
  segments are shorter than native browsing). Accepted residual risk.
- **Citations**: `config/client/sing-box-skeleton.json:78-89`.

### 5.6 `urltest` parameters: 30s, tolerance 50, interrupt existing
- **Decision**: Probe every 30s with `tolerance: 50` ms;
  `interrupt_exist_connections: true`; probe URL
  `https://www.gstatic.com/generate_204`.
- **Why**: 30s is the conventional cadence. Interrupt-existing is essential
  for the "dead tunnel" pattern — without it, long-lived TCP sessions stay
  stuck on a failed outbound.
- **Trade-offs**: Sustained probe-failure cadence is the documented trigger
  for time-windowed DPI flow-learning (§5.7). For single-server-pool clients the
  urltest pool is intentionally tiny (1 visible server) so probe failure
  has no fallback target — accepted because the alternative (multi-server
  pool) spreads the trigger across more flows.
- **Citations**: `config/client/sing-box-skeleton.json:92-99`.

### 5.7 Default `--proto` stays `all`
- **Decision**: `render-config`'s default `--proto` value is `all`. Explicit
  `--proto xhttp` is passed at the call site for clients in the
  high-DPI-pressure egress region.
- **Why**: Flipping the global default would silently strip TCP+vision from
  every render — including for clients where TCP+vision works fine and is
  a useful fallback.
- **Citations**: `scripts/render-config:35`; PR #10 plan-review feedback.

### 5.8 `route_exclude_address` built from VISIBLE servers only
- **Decision**: TUN `route_exclude_address` is computed from the visible
  servers list (`client_render != false`), as `["<host>/32", ...]`.
- **Why**: Excludes server IPs from TUN routing so client → server TCP
  doesn't loop into the TUN. Hidden upstream-only servers are never directly
  connected to from the client, so excluding them was actively wrong.
- **Citations**: `scripts/render-config:158`; PR #10.

### 5.9 Five DNS servers with split-horizon routing
- **Decision**: sing-box's `dns.servers` declares five servers, conditionally
  pruned at render time:
  - `proxy-dns` (UDP 1.1.1.1, `detour: auto`) — final/default, tunneled
  - `direct-dns` (DoH 1.1.1.1, IP literal, no detour) — bootstrap resolver
  - `company-dns` (UDP `${INTERNAL_DNS_1}`, `detour: tailscale|auto`) —
    conditional on `--with-corp-dns`
  - `magicdns` (tailscale endpoint) — conditional on `--with-tailscale`
  - `russia-dns` — see §5.10
- **Why**: Split-horizon DNS: bootstrap before VPN comes up, internal corp
  lookups via tailnet/chain, regional domains via a regional resolver,
  everything else via the VPN exit.
- **Citations**: `config/client/sing-box-skeleton.json:27-78`.

### 5.10 Split-routed regional DNS is DoH (PR #12)
- **Decision**: The split-routed regional resolver uses DoH at
  `dns.comss.one/dns-query` via `type: https`, with
  `domain_resolver: direct-dns` for bootstrap. Replaces a prior plaintext
  UDP entry.
- **Why**: Plaintext UDP queries to the regional resolver were a
  cross-correlation surface — a passive on-path tap could join the DNS log
  with TLS metadata: "this Mac never DNS-resolved CDN-host X but TLS-connects
  to that SNI on a non-CDN IP" was a strong proxy signal even with ASN-match
  SNI camouflage. DoH encrypts the query content; only the DoH endpoint
  hostname is visible.
- **Alternatives considered**: A regional ISP's well-known DNS provider
  was checked first — DoT-only on :853, rejects HTTPS GET on :443
  (verified via curl). Falling back to a community DoH provider with
  multi-IP round-robin and ASN-appropriate vantage.
- **Trade-offs**: Bootstrap dependency on `direct-dns` (which itself routes
  through the chain for resolution). Hard-fail mode: if `dns.comss.one` is
  unreachable, regional lookups fail (no auto-fallback to `proxy-dns` —
  sing-box's `final` only catches unmatched queries, not failed ones).
- **Citations**: `config/client/sing-box-skeleton.json:52-57`; PR #12.

### 5.11 `default_domain_resolver: direct-dns` bootstrap
- **Decision**: The route layer sets `default_domain_resolver: "direct-dns"`.
- **Why**: sing-box ≥1.10 requires explicit bootstrap for hostname-form DNS
  server addresses. Without it, sing-box can't resolve `dns.comss.one`
  because the resolver IS dns.comss.one (chicken-and-egg).
- **Citations**: `config/client/sing-box-skeleton.json:111`.

### 5.12 Route rule order
- **Decision**: Strict ordering in `route.rules`:
  1. Sniff (extracts SNI for subsequent rules)
  2. DNS-hijack (intercept system DNS to sing-box's resolver)
  3. `preferred_by: ["tailscale"]` → tailscale outbound (when present)
  4. Corp CIDRs (`10.0.0.0/8`, `172.16.0.0/12`) → tailscale outbound
  5. `ip_is_private` → direct (LAN traffic)
  6. Regional-domain rules → direct (split routing)
  7. Regional geoip rule-set → direct
  8. `final: auto` → urltest chain
- **Why**: First-match-wins. Sniff first so domain rules can match by SNI;
  DNS hijack second so user-app DNS lookups go through sing-box.
- **Citations**: `config/client/sing-box-skeleton.json:113-145`.

### 5.13 Tailscale-bound rules rewritten when `--with-tailscale` is off
- **Decision**: When the flag is off, `render-config` runs jq passes to:
  - strip the embedded `tailscale` endpoint
  - delete the `magicdns` DNS server and any rule referencing it
  - delete `preferred_by: ["tailscale"]` route rules (no equivalent without
    the endpoint)
  - rewrite remaining `outbound: tailscale` rules to `outbound: auto`
- **Why**: The default (Tailscale-on-exit) wants corp/tailnet IP traffic to
  still tunnel — just via the VPN chain rather than embedded tsnet. Stripping
  the rules entirely would let `10.x` fall through to `direct` and die.
- **Citations**: `scripts/render-config:228-236`.

### 5.14 `company-dns` `detour` follows `--with-tailscale`
- **Decision**: When `--with-corp-dns` is set but `--with-tailscale` is off,
  `company-dns` has its `detour` rewritten from `tailscale` to `auto`.
- **Why**: Symmetric with §5.13. Corp DNS must reach the corporate resolver
  either via embedded tsnet or via the VPN chain to the exit.
- **Citations**: `scripts/render-config:247-251`.

### 5.15 Tailscale + corp-DNS are opt-in flags (default off)
- **Decision**: `--with-tailscale` and `--with-corp-dns` default off.
- **Why**: PR #8 inverted the prior default-on flags. Most use cases don't
  need either; explicit opt-in is clearer.
- **Citations**: `scripts/render-config:32-40`; PR #8.

### 5.16 uTLS fingerprint pinned to `chrome`
- **Decision**: All client outbounds use `utls.fingerprint: "chrome"`.
  xray-core's REALITY outbound matches.
- **Why**: Highest-volume real-traffic JA3/JA4 — blends with the largest
  pool. Hardcoded on both processes (sing-box default + xray's literal) so
  they cannot drift.
- **Trade-offs**: Pinned to whatever uTLS's "chrome" matches at build time —
  known staleness gap as real Chrome's actual fingerprint evolves with each
  release. Mitigation = auto-update sing-box/xray-core.
- **Citations**: `scripts/render-config:184, 304`.

### 5.17 `connect_timeout: 10s` on every outbound
- **Decision**: 10-second TCP connect timeout on both `xhttp-*` (SOCKS) and
  `tcp-*` (VLESS) sing-box outbounds. `direct-dns` uses 5s.
- **Why**: Default DNS timeout (~90s) leaves dead SOCKS/upstream paths
  undetected until probes time out long after. 10s makes urltest mark them
  dead fast.
- **Citations**: `scripts/render-config:167, 177`.

### 5.18 Clash API + Yacd UI (observability)
- **Decision**: `experimental.clash_api` enabled, bound to `127.0.0.1:9090`;
  Yacd UI auto-downloaded.
- **Why**: Force-probe + outbound inspection is useful for debugging.
  Bound to localhost only.
- **Citations**: `config/client/sing-box-skeleton.json:6-15`.

### 5.19 `cache_file` enabled (state persistence)
- **Decision**: sing-box persists DNS cache and selector state across
  restarts.
- **Why**: Avoid losing learned state on every restart.
- **Citations**: `config/client/sing-box-skeleton.json:7-9`.

### 5.20 macOS search domain configured automatically
- **Decision**: `vpn-start` runs `configure-search-domain add
  $COMPANY_DOMAIN` after services start; `vpn-stop` removes it. Falls back
  to the Wi-Fi service when route lookup returns the TUN interface.
- **Why**: macOS-specific — without it, `ssh hostname` doesn't auto-append
  the corp domain.
- **Trade-offs**: macOS-only.
- **Citations**: `scripts/vpn-start`; `scripts/configure-search-domain`.

### 5.21 Geo rule sets fetched via `direct` (no chicken-and-egg)
- **Decision**: SRS-format rule sets (`geoip-{ru}`, `geosite-category-ru`)
  downloaded from `raw.githubusercontent.com` with `download_detour:
  direct`.
- **Why**: Initial rule-set download requires uncensored direct access;
  once cached, works offline. Downloading via the chain would loop on the
  rule sets that determine the routing.
- **Citations**: `config/client/sing-box-skeleton.json:146-161`.

### 5.22 xray-core client config is minimal (SOCKS in + VLESS out only)
- **Decision**: xray-core renders only per-server SOCKS inbounds (`udp:
  true` for DNS/QUIC), one VLESS+REALITY+XHTTP outbound per server, one
  `direct` outbound, and per-server routing rules + `geoip:private →
  direct`.
- **Why**: No TUN, no DNS, no urltest in xray-core — sing-box owns those.
  Keep xray-core's surface minimal.
- **Citations**: `scripts/render-config:257-326`;
  `config/client/xray-xhttp-skeleton.json`.

### 5.23 `xhttpSettings.path` is per-server random; no `host` field
- **Decision**: xray-core's XHTTP outbound sets the per-server `path` from
  `servers.json` (`/<xhttp_path>`) and explicitly does NOT set the `host`
  field.
- **Why**: REALITY derives Host from SNI/address. Setting `host` explicitly
  is a documented break — empirical regression noted in the project's relay
  memory.
- **Citations**: `scripts/render-config:294-298`.

---

## 6. Operations & workflow

### 6.1 Branch → test on the test client → PR → merge (never push to main)
- **Decision**: All changes flow through a feature branch, get validated on
  the test client first, then opened as PR and merged. No direct pushes to
  remote `main`.
- **Why**: Branch protection + review + agent-driven validation catches
  ship-blocking bugs before they hit anyone's working VPN.
- **Citations**: memory `feedback_pr_workflow.md`.

### 6.2 Test on the test client first, never on the local primary
- **Decision**: Every VPN/proxy config change is deployed and validated on
  the test client before touching the primary client.
- **Why**: Breaking the primary client's VPN drops the agent's own
  connectivity mid-task. Test client can be restarted freely.
- **Citations**: memory `feedback_test_on_macold.md`.

### 6.3 Never stop the VPN from the agent session
- **Decision**: The agent never runs `vpn-stop`, kills sing-box, or
  otherwise interrupts the VPN data plane on the user's primary machine.
  Starting a stopped VPN is fine; stopping a running one is not.
- **Why**: The agent's API requests flow through the VPN. Stopping it cuts
  the agent's connectivity mid-task.
- **Citations**: memory `feedback_no_stop_vpn.md`.

### 6.4 RO-only on shared infrastructure
- **Decision**: Never run `systemctl restart`, `tailscale up --reset`,
  `iptables -A/-D`, file deletions, or any mutating command on shared infra
  (e.g., subnet routers). Diagnostic SSH is fine; mutations are not.
- **Why**: Shared infra serves other team members; a restart cascades to
  them.
- **Citations**: memory `feedback_no_mutate_shared_infra.md`.

### 6.5 Update path is full reinstall, NOT in-place re-render
- **Decision**: Updates on installed clients always mean regenerating the
  package on a dev machine and re-running `vpn-install`. The package
  bundles `vpn-start`, `vpn-stop`, `vpn-install`, `configure-search-domain`,
  `render-config`, `validate-config`, the rendered configs, both
  skeletons, and a sanitized `servers.json` (private keys + ssh aliases
  stripped, `client_render:false` entries filtered out), plus plists. It
  does NOT ship `.env` or the full `users.json`. Even though
  `render-config` and skeletons are shipped, re-rendering on the
  installed client is unsupported — the dev machine is the only
  supported render site.
- **Why**: KISS. The installed-client runtime surface is intentionally
  bounded by the dev-side render pipeline; supporting in-place re-render
  would require shipping `.env`/`users.json` (secrets) and would create
  drift.
- **Citations**: `scripts/generate-client-config:172-189`; memory
  `feedback_no_inplace_update.md`; PR #8.

### 6.6 Validate config BEFORE restarting any service (policy)
- **Decision (policy)**: Always run `sing-box check -c config.json` before
  restarting sing-box; run `xray -test -config /etc/xray/config.json`
  (inside the running container) before restarting xray. Stage new
  config to a separate path, validate, then swap.
- **Where this is enforced today**:
  - Client side (`scripts/vpn-start`): pre-flight runs `sing-box check`
    on the rendered config before any launchctl op.
  - User-management side (`scripts/xray-users restart_xray`): runs
    `xray -test` inside the container before restart.
  - Plan-driven server rollouts: backup → stage → `xray -test` on
    `/opt/xray/config-new.json` → swap → restart, with auto-rollback on
    container-unhealthy.
- **Where this is NOT enforced**: `scripts/deploy.sh start_container()`
  writes `/opt/xray/config.json` directly and then runs `docker compose
  restart xray` without a `xray -test` pre-check. The container's own
  startup will refuse a syntactically invalid config and report
  unhealthy via the healthcheck, but there's a window where a bad config
  takes the prior healthy container down. **TODO** (see §8): add a
  validate-before-restart step to `deploy.sh`.
- **Why**: A bad config can break VPN/network connectivity and lock the
  operator out. Cheap pre-check vs costly recovery.
- **Citations**: AGENTS.md § Config Change Safety; `scripts/vpn-start`;
  `scripts/xray-users:59-72`; plan
  `docs/plans/20260514-asn-match-sni-and-xhttp-only-chain.md` § Testing
  Strategy.

### 6.7 Backup before destructive ops
- **Decision**: Before mutating remote `/opt/xray/config.json` or local
  `servers.json`, write a timestamped `.bak` first.
- **Why**: Cheap rollback path. Required for the documented
  validate-then-swap-then-restart pattern in plan rollouts.
- **Citations**: AGENTS.md; plan
  `docs/plans/20260514-asn-match-sni-and-xhttp-only-chain.md`.

### 6.8 Resolve PR review threads after pushing fixes
- **Decision**: After pushing a fix that addresses a Copilot or human
  review comment, resolve each corresponding GitHub review thread via the
  GraphQL `resolveReviewThread` mutation.
- **Why**: Keeps the PR's open-thread count accurate. Genuine
  open-question threads get an explicit reply explaining why instead of
  being silently resolved.
- **Citations**: memory `feedback_resolve_pr_threads.md`.

### 6.9 Wait for Copilot + CI after every PR creation
- **Decision**: After `gh pr create`, do not report "PR ready" until both
  Copilot review is submitted AND all CI checks have settled.
- **Why**: Copilot has caught ship-blocking bugs in this project (notably
  the `// null` jq footgun) that dual-review missed. CI's PII scan blocks
  at the org-policy layer.
- **Citations**: memory `feedback_post_pr_wait_copilot_and_ci.md`.

### 6.10 Anonymous commit author identity enforced in CI
- **Decision**: Every commit on a PR must have an author email ending in
  the GitHub-issued anonymous-noreply suffix. The `author-check` workflow
  fails the PR otherwise.
- **Why**: Avoid PII (real names, corp emails) in commit metadata.
- **Citations**: `.github/workflows/author-identity.yml`; PR #7.

### 6.11 Gitleaks PII/secrets scan in CI
- **Decision**: Every PR runs `gitleaks` via a reusable workflow with a
  private config delivered through a repo secret. Catches home-directory
  paths, secrets, and fleet-specific patterns.
- **Why**: PR-gated PII enforcement. Private config keeps the pattern list
  out of the public repo (the patterns themselves can leak intent).
- **Citations**: `.github/workflows/pii-scan.yml`.

### 6.12 PII scrubbing in commit messages and committed files
- **Decision**: Strip jurisdictional labels, specific operator names,
  fleet-identifying IP literals, and home-directory paths from anything
  committed. The local memory keeps that context; the repo does not.
- **Why**: Public/shared repo. Gitleaks enforces; humans must self-edit.
- **Trade-offs**: Code lacks self-contained jurisdictional context. Local
  memory carries the ground truth.

### 6.13 Never `--amend` commits; always new commit
- **Decision**: Even local-only commits get followed up with a new commit
  for fixes. Exception: explicit user request.
- **Why**: Pre-commit hook failure ≠ commit happened; amending would
  modify the wrong commit. Standard rule from project + global CLAUDE.md.

### 6.14 Force-push allowed on unmerged feature branches; always `--force-with-lease`
- **Decision**: Force-push is acceptable for cleanup on a not-yet-merged
  branch. Always `--force-with-lease`, never bare `--force`.
- **Why**: Cleanup of bad commits / PII scrubs / scope fixes is more
  important than preserving local exploration. `--force-with-lease`
  prevents accidentally wiping a teammate's commits.

### 6.15 PR title and body must match actual diff scope
- **Decision**: PR title and body must describe the actual diff vs base.
  If scope changes (e.g., rebased to drop commits), edit the PR
  description.
- **Why**: Reviewers need accurate scope to review efficiently. Caught on
  PR #12 first review when the branch was stacked on PR #11.

### 6.16 PR description includes validation evidence
- **Decision**: Every PR has a `## Test plan` section listing concrete
  checks executed (tcpdump captures, curl probes, server-state checks,
  config validations).
- **Why**: Documents what's been exercised; reviewers don't guess. Pattern
  visible across PRs #6, #8, #9, #10, #11, #12.

### 6.17 Single-probe validation (no probe-spam test loops)
- **Decision**: Post-deploy validation uses single curl + single force-probe
  per outbound, NOT 5-probes-3s-apart loops.
- **Why**: 5-10 failures in quick succession against the same 5-tuple is
  the documented DPI flow-learning trigger. Validation itself must not
  burn the path being validated.
- **Citations**: plan `docs/plans/20260513-orca-hel-sni-gcore-swap.md`;
  memory `project_ru_dpi_aws_sni.md`.

### 6.18 Bridge-via-test-inbound for risky SNI swaps
- **Decision**: For server SNI/key changes, deploy a parallel test
  container on a sandbox port with the new config; migrate the test client
  to the test inbound first; swap prod; migrate back.
- **Why**: Avoids the 10-second client/server mismatch window where
  REALITY falls back to its synthetic cert and silently breaks the chain.
- **Citations**: plan `docs/plans/20260513-orca-hel-sni-gcore-swap.md`.

### 6.19 Plans live in `docs/plans/<YYYYMMDD>-<slug>.md`
- **Decision**: Non-trivial changes get a markdown plan in
  `docs/plans/` with date prefix. Plans cover Overview, Context,
  Development Approach, Testing Strategy, Progress Tracking, Solution
  Overview, Technical Details, Rollback.
- **Why**: Auditable rollouts; rollback paths and validation checkpoints
  baked in.

### 6.20 Dual-review (`/devops:dual-review`) before opening risky PRs
- **Decision**: For non-trivial PRs, run Codex + a fresh Opus subagent in
  parallel against the branch diff before opening the PR. Merge findings,
  address everything material, then open.
- **Why**: Catches ship-blocking bugs (scope mismatch, jq footguns, wrong
  endpoint hostnames) at the cheapest stage. PR #10 and PR #12 each had
  material findings flagged this way.

### 6.21 Dialectic for high-stakes config choices
- **Decision**: When intuition disagrees with received wisdom, run a
  parallel thesis + antithesis analysis grounded in actual configs.
- **Why**: PR #11's `tcp_fast_open` flip from `true` to `false` came out
  of a dialectic — the antithesis won on TCP/TLS-layer fingerprint
  coherence grounds.

### 6.22 Codex consultation for hard technical questions
- **Decision**: `/thinking-tools:ask-codex` for read-only analysis when
  explicitly asked, or as a last resort after 4+ failed debugging attempts.
- **Why**: Second LLM perspective. Caught the SNI/IP correlation hypothesis
  that unlocked the upstream-only-exit path.

### 6.23 Two-sided tcpdump for dead-tunnel diagnosis
- **Decision**: When TCP handshakes succeed but data doesn't flow ("dead
  tunnel"), capture on BOTH endpoints simultaneously at the same
  wall-clock time, then filter to PSH-only and compare patterns to
  localise the drop.
- **Why**: ICMP, `nc -vz`, and single-sided captures cannot distinguish
  payload-aware in-path drop vs application hang vs client-side
  dead-write. The asymmetry between "PSHs left the client interface"
  and "ZERO PSHs arrived on the server" is uniquely diagnostic. ~2 hours
  were lost on a previous incident to single-sided red-herring chasing
  (MTU / TFO / MSS) before two-sided capture identified the actual cause.
- **Citations**: memory `feedback_twosided_tcpdump_dead_tunnel.md`.

### 6.24 Mac-reboot-first when embedded Tailscale loses corporate connectivity
- **Decision**: When sing-box's embedded `tsnet` loses corporate subnet
  connectivity (banner-exchange timeouts, corp DNS hangs, internal API
  hangs), suggest `sudo reboot` on the Mac BEFORE any config debugging,
  version reverting, or server-side investigation.
- **Why**: Empirically, embedded `tsnet` on macOS gets into wedged states
  that survive sing-box restarts but a kernel-level reboot fixes in 2
  minutes. A previous incident burnt ~5 hours chasing every plausible
  theory (sing-box version, TFO, route_exclude, CGNAT, ACL/tag,
  subnet-router state, netmap pollution) — none were the fix; a clean
  Mac reboot was.
- **Citations**: memory `feedback_tailscale_subnet_router_debug.md`.

### 6.25 `tsnet "NoState. Ignoring authkey"` is benign startup noise
- **Decision**: Treat the `tailscale: state is NoState. Ignoring
  authkey` log line as informational. Do NOT recommend deleting
  `~/.local/share/sing-box-tailscale/tailscaled.state` — that rotates the
  Tailscale node IP and breaks ACLs.
- **Why**: Empirically benign — login was fine in every observed
  occurrence; the log line precedes a successful tsnet startup. A prior
  agent reading the line at face value recommended a destructive cleanup
  that broke node-IP-pinned ACLs.
- **Citations**: memory `feedback_tsnet_nostate_benign.md`.

### 6.26 Dual-review BEFORE every PR creation
- **Decision**: Run `/devops:dual-review` (Codex + fresh Opus subagent
  in parallel against the branch diff) BEFORE `gh pr create` on every
  non-trivial PR. Docs-only / single-line fixes are exempt by judgment;
  when in doubt, run it.
- **Why**: Pre-flight catches material findings at the cheapest review
  stage — scope mismatch, wrong endpoint hostnames, internal
  inconsistencies — before reviewer cycles burn. PR #12 caught both a
  silent stacked-branch issue AND a non-existent DoH endpoint this way.
- **Trade-offs**: ~5-10 min added per PR. Massively cheaper than a
  post-merge regression.
- **Citations**: memory `feedback_dual_review_before_pr.md`.

---

## 7. Residual risks & accepted trade-offs

### 7.1 Latency-delta active-probe vulnerability
- **Decision**: Accept as a residual risk; no in-protocol mitigation.
- **Why**: REALITY's MirrorConn adds the dest-RTT to every probe response.
  A native HTTPS probe to `dest` directly has one RTT; a probe to the
  REALITY server has two. The delta is detectable.
- **Mitigation**: Pick `dest` sites whose handshakes are intrinsically slow
  so the delta is in the noise. Long-term: would require a different
  protocol layer (no REALITY tweak fixes this).

### 7.2 No ECH (Encrypted Client Hello)
- **Decision**: Not configured. SNI is plaintext on the wire.
- **Why**: ECH requires `dest` cooperation (published ECH config in DNS).
  Most `dest` choices don't publish ECH. Plus ECH itself is fingerprintable
  in some jurisdictions today.
- **Trade-offs**: SNI is intentionally visible — it IS the camouflage. The
  observer is meant to see "client visiting <camouflage site>".

### 7.3 No JA3/JA4 rotation across the fleet
- **Decision**: All servers + clients pin `fingerprint: "chrome"`.
- **Why**: Per-server variation would break cross-fleet correlation
  defence but adds operational complexity. Accepted as residual.
- **Future**: per-server `fingerprint` field in `servers.json` could
  vary across servers without scripted rotation.

### 7.4 REALITY private_key, short_id, xhttp_path lifecycle
- **Decision**: Generated once at deploy, persisted in `servers.json`,
  never rotated unless server is redeployed fresh.
- **Why**: No automated rotation infrastructure; rotation requires
  coordinated client + server config swap.
- **Future**: `make rotate-server-creds NAME=<server>` covering
  `xhttp_path`, `short_id`, and (with explicit opt-in) the x25519 keypair.
  shortIds is an array field so brief overlap windows are possible.
- **Severity**: REALITY-private-key rotation is a hard cutover (clients
  with old key can't connect). Quarterly rotation NOT recommended for
  `private_key`; emergency rotation runbook is the goal.

### 7.5 urltest cadence is a documented DPI trigger
- **Decision**: Accept. The 30s + `interrupt_exist_connections: true`
  pattern is exactly the trigger described in the project's DPI memory.
  Single-outbound urltest pool amplifies this (no fallback target).
- **Mitigation under consideration**: probe interval jitter, longer
  intervals, supervisor that pauses the urltest pool after N consecutive
  failures.

### 7.6 Single point of failure on the chain
- **Decision**: Accept. The relay is the only path to the upstream-only
  exit; if it goes down, that exit is unreachable.
- **Why**: Intentional — the user-stated intent is to keep the first hop
  within the same ASN as the camouflage SNI, no border-crossing on the
  first hop. The direct exit (other server) is independent and survives a
  relay outage; only chain-dependent traffic (corp via the upstream's
  Tailscale) is affected.

### 7.7 Exit server still sniffs (out of scope of PR #11)
- **Decision**: Exit template's `sniffing.enabled: true` is unchanged.
- **Why**: Different threat model — exit is in a lower-risk jurisdiction.
  Same metadata-leak concern applies but is lower-priority.
- **Future**: Flip to `false` if exit-side sniffing-based routing is
  never needed.

### 7.8 TUN MTU 1280 is a packet-size fingerprint
- **Decision**: Accept. 1280 is the IPv6-min standard sing-box default;
  matching native Mac MTU (1500) would risk fragmentation on some paths.
- **Future**: experiment with 1420 (WireGuard default — but that's its own
  fingerprint).

### 7.9 Plaintext bootstrap of `direct-dns` over the chain
- **Decision**: `direct-dns` (DoH to 1.1.1.1) has no `detour` set,
  so it routes through the chain. First-time DoH bootstrap (resolving
  `dns.comss.one` for `russia-dns`) goes through the urltest pool, which
  can add 5-10 seconds on cold start if the chain is degraded.
- **Future**: rename `direct-dns` to `chain-dns` to match reality, or
  add an alternative IP-literal bootstrap that genuinely goes direct via
  the host network.

---

## 8. Known TODOs

- `xray-users add` silent-fail wrap (`|| { failed+=("$host"); }`) — script
  exits early under `set -euo pipefail` if any inner command fails;
  manual workaround in use.
- `make rotate-server-creds NAME=<server>` covering `xhttp_path`,
  `short_id`, and (opt-in) the x25519 keypair.
- urltest probe-failure backoff supervisor.
- Exit-template `sniffing: false` (mirror PR #11 to the exit template).
- Documented bootstrap path that genuinely doesn't loop through the
  chain (rename or restructure `direct-dns`).
- `deploy.sh start_container()`: add `xray -test -config
  /etc/xray/config.json` between config upload and `docker compose
  restart` so the validate-before-restart policy (§6.6) is enforced on
  the deploy path, not only on the client and user-management paths.

---

## Decision count

- §1 Protocol & transport: 7
- §2 Server architecture: 12
- §3 Server hardening: 11
- §4 SNI selection: 4
- §5 Client architecture: 23
- §6 Operations & workflow: 26
- §7 Residual risks / accepted trade-offs: 9
- §8 Known TODOs: 6

Total: 92 distinct decisions + 9 residuals + 6 open follow-ups,
distilled from ~228 raw findings across four parallel-agent decision
sweeps with deduplication, then pre-PR-dual-reviewed for factual drift
and PII before merge.
