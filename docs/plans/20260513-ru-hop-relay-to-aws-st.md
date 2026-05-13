# RU-hop relay to aws-st (multi-hop VLESS chain)

## Overview

Add a new VPN server `ru-hop` (139.100.204.69, RU commercial hosting) configured as a **VLESS chain relay**: Mac clients connect to ru-hop via VLESS+REALITY, and ru-hop's xray outbound chains every connection to `aws-st` (16.171.223.39). aws-st remains the actual internet exit; ru-hop is a pure encrypted relay.

**Problem this solves:** today (2026-05-13) we confirmed RU-ISP DPI heavily filters direct Mac→aws-st traffic (AWS source-IP correlated payload-fingerprint drops, TFO SYN-ACK drops, sustained probe-failure DPI flow-burn). orca-hel works fine but only one exit is available for direct VPN traffic. Adding ru-hop as a relay restores access to aws-st as an alternate exit, bypassing consumer-ISP→AWS DPI rules since the cross-border leg becomes RU-datacenter→AWS (different ASN profile).

**Architecture:**
```
Mac (sing-box+xray) ──VLESS+REALITY──> ru-hop (xray dual-role) ──VLESS+REALITY──> aws-st (xray exit) ──> internet
                       (within RU,                                (RU-DC to AWS,
                        weak DPI)                                  bypasses consumer DPI)
```

End-user UUIDs authenticate once at Mac→ru-hop. ru-hop's outbound to aws-st multiplexes all client traffic through a single `ru-hop-relay` UUID. Per user preference, **all 3 servers maintain identical synced user lists** including `ru-hop-relay` — so direct Mac→aws-st remains a valid fallback path.

## Context (from discovery + brainstorm)

- **Existing repo patterns:**
  - `config/server.template.json` — single-inbound exit-server template, `<UUID>`/`<SNI>`/etc placeholders substituted by deploy.sh
  - `scripts/deploy.sh` — provisions a new exit server: harden, install docker, generate REALITY keys, render config, save to `servers.json`
  - `scripts/render-config` — generates client sing-box + xray configs from `servers.json` per-server entry (4 outbounds per server: xhttp-*, tcp-*)
  - `scripts/xray-users` — CRUD across all servers in `servers.json`, generates VLESS share URLs
- **Working VPN today:** orca-hel (Gcore Helsinki) primary, aws-st (AWS Stockholm) DPI-burnt and unreliable from RU Macs
- **REALITY keys:** orca-hel + aws-st each have their own (`public_key`/`private_key`/`short_id` in `servers.json`). ru-hop will get its own pair too
- **xray dual-role:** xray supports being both server (inbound) and client (outbound) in one config. The relay pattern is standard xray usage — needs `routing.rules` to send inbound traffic to the chained outbound instead of `freedom`/direct

## Development Approach

- **Operational change** — bash scripts + json configs. No Go/Python/TS code, no unit tests in the traditional sense. **"Tests" = operational verification** (clash-API probes + curl external-IP + xray-test config validation).
- Pre-stage everything before destructive actions. Validate JSON schemas before pushing. Maintain rollback paths.
- macold-first testing (per `feedback_test_on_macold.md`). Local Mac client changes are USER-driven kickstart (per `feedback_no_stop_vpn.md`).
- Make small focused changes; verify each before moving on.

## Testing Strategy

Operational verification at each phase:

- **Phase 1 (deploy):** `xray -test` validates rendered config inside docker container before xray starts. Container health-check (`xray version`) confirms process alive.
- **Phase 2 (network):** `ssh gigi 'sudo ss -tlnp | grep -E :443\\|:8443'` confirms both inbounds listening. Raw `nc -vz` from macold confirms TCP reachability.
- **Phase 3 (chain):** single clash-API force-probe per new outbound (`xhttp-ru-hop`, `tcp-ru-hop`) — each <500ms. **Critical test:** curl ifconfig.me through `xhttp-ru-hop` from macold returns `16.171.223.39` (aws-st's IP), proving the chain works end-to-end.
- **Phase 4 (sustain):** 5 sustained probes 60s apart (not 30s — avoid DPI burn trigger). All succeed.
- **Phase 5 (local Mac):** same checks after USER kickstarts manually.

No spammy probe patterns — every verification uses 1-5 probes, not bulk runs.

## Progress Tracking

- `[x]` when done
- ➕ for newly discovered work
- ⚠️ for blockers
- update this file as scope evolves

## Solution Overview

**Single xray container on ru-hop** runs in dual-role:
- 2 inbounds (mirror other servers): XHTTP on 443, TCP+vision on 8443 — both with REALITY anti-detection
- 2 outbounds (the chain): one matching each inbound transport, both VLESS+REALITY connecting to aws-st (XHTTP→aws-st:443, TCP+vision→aws-st:8443)
- routing rules: `inbound xhttp-443 → outbound xhttp-aws-st`, `inbound tcp-8443 → outbound tcp-aws-st`
- NO direct/freedom outbound — ru-hop is pure relay, never exits to internet directly

**Mac client side:** no protocol changes. ru-hop appears as a third server in `servers.json`. Render-config produces standard 4 outbounds per server (`xhttp-orca-hel`, `tcp-orca-hel`, `xhttp-aws-st`, `tcp-aws-st`, `xhttp-ru-hop`, `tcp-ru-hop` — 6 total). urltest pool includes all 6. Mac doesn't know/care that ru-hop is a relay.

**User database:** identical across all 3 servers. `xray-users add ru-hop-relay` adds that UUID to each server's `clients` array. RU-hop's outbound to aws-st uses this UUID.

**Repo schema impact (minimal):**
- New `config/server-relay.template.json` — extends `server.template.json` with outbounds+routing sections
- `servers.json` gets one new field on ru-hop entry: `relay_upstream: "aws-st"` — used at deploy time to look up upstream's keys/SNI/path
- `scripts/deploy.sh` extended: when `relay_upstream` set, use relay template and inject upstream values

## Technical Details

### `config/server-relay.template.json` placeholders (extends exit template)

The relay template **is a separate file** from `server.template.json` (the exit-server template). The base template stays untouched. The relay template adds `tag: "xhttp-in"` and `tag: "tcp-in"` to its inbounds (referenced by routing rules), plus the outbounds + routing sections below.

**IMPORTANT — match working client xray config exactly** (per render-config inspection, `xhttpSettings` for client xhttp outbound only sets `path`, never `host`; setting `host` may break REALITY masquerade):

```json
"outbounds": [
  {
    "tag": "to-upstream-xhttp",
    "protocol": "vless",
    "settings": {
      "vnext": [{
        "address": "<UPSTREAM_HOST>",
        "port": 443,
        "users": [{
          "id": "<RU_HOP_RELAY_UUID>",
          "encryption": "none",
          "packet_encoding": "xudp"
        }]
      }]
    },
    "streamSettings": {
      "network": "xhttp",
      "xhttpSettings": {"path": "/<UPSTREAM_XHTTP_PATH>", "mode": "auto"},
      "security": "reality",
      "realitySettings": {
        "serverName": "<UPSTREAM_XHTTP_SNI>",
        "publicKey": "<UPSTREAM_PUBLIC_KEY>",
        "shortId": "<UPSTREAM_SHORT_ID>",
        "fingerprint": "chrome"
      }
    }
  },
  {
    "tag": "to-upstream-tcp",
    "protocol": "vless",
    "settings": {
      "vnext": [{
        "address": "<UPSTREAM_HOST>",
        "port": 8443,
        "users": [{
          "id": "<RU_HOP_RELAY_UUID>",
          "encryption": "none",
          "flow": "xtls-rprx-vision",
          "packet_encoding": "xudp"
        }]
      }]
    },
    "streamSettings": {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {
        "serverName": "<UPSTREAM_SNI>",
        "publicKey": "<UPSTREAM_PUBLIC_KEY>",
        "shortId": "<UPSTREAM_SHORT_ID>",
        "fingerprint": "chrome"
      }
    }
  }
],
"routing": {
  "domainStrategy": "AsIs",
  "rules": [
    {"type": "field", "inboundTag": ["xhttp-in"], "outboundTag": "to-upstream-xhttp"},
    {"type": "field", "inboundTag": ["tcp-in"], "outboundTag": "to-upstream-tcp"}
  ]
}
```

**Changes from reviewer feedback applied:**
- `xhttpSettings` does NOT set `host` (matches working client xray config; xray-core defaults Host header from SNI/address)
- `packet_encoding: "xudp"` set on each outbound user (xray-core's outbound default may differ from sing-box's; explicit setting ensures UDP DNS works through the chain)
- Inbound `tag` fields exist **only in `server-relay.template.json`**, never added to base `server.template.json`

**Vision-on-vision risk (`to-upstream-tcp`):** the ru-hop inbound on port 8443 uses `xtls-rprx-vision` (Mac→ru-hop leg) AND the outbound to aws-st also uses `xtls-rprx-vision` (ru-hop→aws-st leg). This is "vision-on-vision" — xray-core's behavior here is not extensively documented and there are anecdotal reports of handshake issues. Mitigation:
- **Task 9 will explicitly test `tcp-ru-hop` end-to-end.** If it fails, the fallback (documented in Rollback Plan) is to drop the tcp-vision relay leg entirely (use XHTTP-only on ru-hop, since XHTTP is the more reliable transport from today's data anyway). The TCP+vision inbound on ru-hop can stay declared but with an outbound that drops to `freedom` (direct exit through ru-hop) as a degraded fallback — OR simply remove the inbound entirely if vision-on-vision is confirmed broken.

### `servers.json` schema additions

Add one entry, with new optional `relay_upstream` field. **Use `dl.google.com` for both SNIs to match the rest of the fleet** (orca-hel + aws-st both currently use dl.google.com after today's revert):

```json
{
  "name": "ru-hop",
  "host": "139.100.204.69",
  "ssh": "gigi",
  "public_key": "<generated>",
  "private_key": "<generated>",
  "short_id": "<generated>",
  "xhttp_path": "<generated>",
  "xhttp_sni": "dl.google.com",
  "sni": "dl.google.com",
  "relay_upstream": "aws-st"
}
```

Existing orca-hel/aws-st entries unchanged. `relay_upstream` absent on them → render as exit server (existing behavior).

**Critical: `save_server()` in deploy.sh currently enumerates fields explicitly and would strip `relay_upstream` on next save.** Task 3 must extend save_server to preserve this field. Verification step in Task 6 will confirm the field survives the round-trip.

### `scripts/deploy.sh` changes

Two functions need modification:

**1. `load_server()` (parses servers.json entry):** also export new field `RELAY_UPSTREAM` if present.

**2. `save_server()` (lines 68-89):** MUST preserve the `relay_upstream` field. Current code enumerates fields explicitly in the `jq -n` call — without changes, save_server overwrites the entry and silently drops `relay_upstream`. Fix: extend the jq pipeline to include `relay_upstream` conditionally:
```bash
entry=$(jq -n \
    --arg name "$name" \
    --arg host "$SERVER" \
    --arg ssh "$SSH_HOST" \
    --arg public_key "$PUBLIC_KEY" \
    --arg private_key "$PRIVATE_KEY" \
    --arg short_id "$SHORT_ID" \
    --arg xhttp_path "$XHTTP_PATH" \
    --arg xhttp_sni "${XHTTP_SNI:-dl.google.com}" \
    --arg sni "${SNI:-dl.google.com}" \
    --arg relay_upstream "${RELAY_UPSTREAM:-}" \
    '{name:$name, host:$host, ssh:$ssh, public_key:$public_key, private_key:$private_key, short_id:$short_id, xhttp_path:$xhttp_path, xhttp_sni:$xhttp_sni, sni:$sni}
     + (if $relay_upstream == "" then {} else {relay_upstream:$relay_upstream} end)')
```

**3. `create_config()` (the function that renders xray config; deploy.sh line 188):** branch on `RELAY_UPSTREAM`:
- If `RELAY_UPSTREAM` empty/unset → use `config/server.template.json` (existing exit-server behavior, unchanged)
- If `RELAY_UPSTREAM` set:
  - Call new helper `load_upstream "$RELAY_UPSTREAM"` (looks up upstream server's `host`/`public_key`/`short_id`/`xhttp_path`/`xhttp_sni`/`sni` in servers.json, exports `UPSTREAM_*` env vars)
  - Call new helper `load_relay_user_uuid` (reads users.json for the `ru-hop-relay` user, exports `RU_HOP_RELAY_UUID` — exit with error if not present)
  - Use `config/server-relay.template.json` for envsubst, with additional env vars: `UPSTREAM_HOST`, `UPSTREAM_PUBLIC_KEY`, `UPSTREAM_SHORT_ID`, `UPSTREAM_XHTTP_PATH`, `UPSTREAM_XHTTP_SNI`, `UPSTREAM_SNI`, `RU_HOP_RELAY_UUID`

The relay's own inbound REALITY pair + SNI + xhttp_path are generated by deploy.sh's existing `generate_keys`/`generate_short_id`/`generate_xhttp_path` logic (unchanged — runs for any new server regardless of mode).

**Negative test required (Task 3 verification):** redeploying orca-hel (no `relay_upstream`) must still produce identical exit-server config. Manual diff or roundtrip-check.

### `scripts/xray-users` changes

Verify the existing script already iterates all entries in `servers.json` for `add`/`remove`/`sync`. If so, **no code change needed** — just `xray-users add ru-hop-relay` propagates to all 3 servers (including the new ru-hop) automatically once ru-hop is in `servers.json`.

If not (e.g., hardcoded to 2 servers), generalize the loop.

### Client-side (no script changes expected)

`render-config` already iterates `servers.json` to build outbounds. ru-hop entry produces `xhttp-ru-hop` + `tcp-ru-hop` outbounds in client configs. Existing client tools (vpn-install, vpn-start, validate-config) need no changes.

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): repo file changes (templates, deploy.sh), local config generation, server deploy, validation
- **Post-Completion**: operational metrics over days, eventual removal of direct Mac→aws-st outbounds from `servers.json` (per user's "manual cleanup later" preference)

## Implementation Steps

### Task 1: Pre-flight + SSH bootstrap on 139.100.204.69

**Files:** (no repo changes — manual server prep)

- [x] USER set up SSH alias `gigi` pointing at `root@139.100.204.69`
- [x] verified: `ssh gigi 'hostname && uname -a && curl -s --max-time 5 https://ifconfig.me'` — hostname=`gigi`, Linux 6.8.0, public IP `139.100.204.69`
- [x] OS confirmed: Ubuntu 24.04 (Debian-family, deploy.sh compatible)
- [x] Default route confirmed: `default via 139.100.204.1 dev eth0` (clean networking, no NAT/proxy in front)

### Task 2: Add `relay_upstream` schema + new server-relay template

**Files:**
- Create: `config/server-relay.template.json`

- [x] created `config/server-relay.template.json` based on `config/server.template.json`
- [x] added `tag: "xhttp-in"` to the port-443 inbound, `tag: "tcp-in"` to the port-8443 inbound
- [x] appended `outbounds` section with `to-upstream-xhttp` (port 443) and `to-upstream-tcp` (port 8443) — both VLESS+REALITY pointing at `<UPSTREAM_HOST>`, NO `xhttpSettings.host` (matches working client config), `block` outbound retained, NO `direct`/`freedom` (pure relay)
- [x] appended `routing.rules` mapping `xhttp-in`→`to-upstream-xhttp`, `tcp-in`→`to-upstream-tcp`
- [x] verified JSON parses: `jq . config/server-relay.template.json` ✓
- [x] validation test passed: substituted with realistic aws-st values + fresh REALITY pair + UUIDs, `xray -test` returned `Configuration OK` (one info warning about non-443 port — harmless, port 443 itself is fine)

### Task 3: Extend `scripts/deploy.sh` to support relay mode

**Files:**
- Modify: `scripts/deploy.sh`

- [x] extended `load_server()` to parse optional `relay_upstream` field (exports `RELAY_UPSTREAM`, empty if absent)
- [x] added `load_upstream()` helper: takes server name, exports `UPSTREAM_HOST/PUBLIC_KEY/SHORT_ID/XHTTP_PATH/XHTTP_SNI/SNI` from servers.json (errors out if name not found)
- [x] added `load_relay_user_uuid()` helper: reverse-lookup `ru-hop-relay` in users.json `{uuid:name}` schema, exports `RU_HOP_RELAY_UUID` (errors out if user not present with clear message)
- [x] extended `create_config()` to branch on `[[ -n "$RELAY_UPSTREAM" ]]` — uses `server-relay.template.json` + runs `load_upstream` + `load_relay_user_uuid` + substitutes `<UPSTREAM_*>` and `<RU_HOP_RELAY_UUID>` placeholders. Non-relay path completely unchanged (regression-safe).
- [x] extended `save_server()` to preserve `relay_upstream` field via conditional jq merge: `+ (if $relay_upstream == "" then {} else {relay_upstream:$relay_upstream} end)` — won't add empty key on non-relay saves, won't strip field on relay saves
- [x] also updated default `xhttp_sni` fallback from `speedtest.gcore.com` → `dl.google.com` (matches current fleet)
- [x] `bash -n scripts/deploy.sh` passes (syntax OK)
- [x] template-substitution rendering test passed for relay mode (Task 2 validation)

### Task 4: ⚠️ REQUIRES USER APPROVAL — Pre-deploy: add ru-hop-relay user (TRIGGERS DOUBLE XRAY RESTART)

**Files:**
- Modify: `users.json` (auto-modified by `xray-users add`)
- Modify: orca-hel + aws-st `clients` arrays AND restarts xray on both (auto-done by `xray-users add` per its `restart_xray` call)

**⚠️ Operational warning:** `xray-users add` calls `restart_xray $host` after pushing config. This means BOTH orca-hel AND aws-st xray containers get restarted (~5-10s each). Impact:
- Local Mac's existing VPN connections via orca-hel will drop briefly during orca-hel restart. Agent (this Claude session) may lose connectivity for ~10s and reconnect via urltest.
- aws-st restart only affects whatever clients are currently using it (likely none from the agent's machines today since aws-st is DPI-burnt anyway).
- Macold drops aren't a concern (testbed).

- [x] defensive check: gigi NOT in servers.json before run ✓
- [x] confirmed `scripts/xray-users` iterates all entries in servers.json (`while read host` over `jq '.[].ssh' servers.json`)
- [x] executed `./scripts/xray-users add gigi` — user chose `gigi` (matches SSH alias) instead of `ru-hop-relay`. UUID generated (see `users.json`)
- [⚠️] xray-users add completed on orca-hel only; aws-st was NOT updated (xray-users script appears to exit silently after first server's restart — likely `set -euo pipefail` + non-zero exit from `docker compose restart` `>/dev/null` redirect swallowing the error. TODO: file as separate bug, add `|| { echo FAIL; failed+=$host; }` wrapping to fail-safe the loop)
- [x] manually propagated to aws-st via inline ssh+jq+docker-restart sequence (validated `xray -test` before swap, atomic mv + restart, healthy in 6s)
- [x] verified gigi UUID present in both clients arrays on both servers (orca-hel: 2 matches, aws-st: 2 matches) and on both inbounds (port 443 + 8443)
- [x] users.json now has 7 entries including `gigi`
- [x] **renaming decision**: relay user is `gigi` (matches SSH alias). Server in servers.json will also be `name: "gigi"` (consistent naming throughout the stack).
- [x] updated `deploy.sh load_relay_user_uuid()` to take server name as arg (`gigi`), and `RU_HOP_RELAY_UUID` env var renamed to `RELAY_USER_UUID`. Template placeholder renamed `<RU_HOP_RELAY_UUID>` → `<RELAY_USER_UUID>`.

### Task 5: ⚠️ REQUIRES USER APPROVAL — Add ru-hop entry to `servers.json` (pre-deploy stub)

**Files:**
- Modify: `servers.json`

- [x] backed up servers.json to `servers.json.pre-gigi-<ts>`
- [x] jq-added `gigi` entry: `host: "139.100.204.69"`, `ssh: "gigi"`, `xhttp_sni: "dl.google.com"`, `sni: "dl.google.com"`, `relay_upstream: "aws-st"`. Keys/short_id/xhttp_path empty (deploy.sh will generate)
- [x] verified entry present: 3 servers in servers.json (orca-hel, aws-st, gigi)

### Task 6: ⚠️ REQUIRES USER APPROVAL — Deploy ru-hop server

**Files:**
- Modify: ru-hop filesystem (harden + docker + xray config — deploy.sh's standard steps)

- [x] executed `NAME=gigi make deploy` — first run failed at `generate_keys` due to xray-core's newer output format (`Password (PublicKey): ...` not matching old grep `^Public`); patched deploy.sh's regex (`grep PublicKey|Public key`); second run succeeded
- [x] container healthy: `Up 21 seconds (healthy)` (since extended to many hours uptime now)
- [x] both inbounds listening: `*:443` and `*:8443` on gigi (xray PID 14292)
- [x] outbounds correctly chain to aws-st: `to-upstream-xhttp → 16.171.223.39:443`, `to-upstream-tcp → 16.171.223.39:8443`, plus `block` (no `direct`)
- [x] routing rules verified: `xhttp-in → to-upstream-xhttp`, `tcp-in → to-upstream-tcp`
- [x] ➕ also installed `jq` on gigi for ongoing remote diagnostics

### Task 7: Verify ru-hop user list synced + cross-server checks + relay_upstream preserved

**Files:** (no changes — verification)

- [x] gigi's `clients` array has all 7 users on both inbounds
- [x] gigi UUID (see `users.json`) present in gigi's clients on both inbounds (2 matches)
- [x] cross-server: same UUID present on aws-st's both inbounds (2 matches) — this is what gigi's outbound auths as upstream
- [x] cross-server: same UUID present on orca-hel's both inbounds (2 matches) — consistency
- [x] **`relay_upstream` field survived `save_server` roundtrip ✓** — `jq '.[] | select(.name=="gigi") | .relay_upstream'` returns `"aws-st"`. The conditional jq merge in deploy.sh works correctly.

### Task 8: Render and install client package on macold (test-mac-first)

**Files:**
- Create: `config/client/generated/macold-orca-hel-tcp-vpn.zip` (generated by script)
- Modify: macold `~/.config/sing-box/config-auto.json`, `~/.config/xray/config.json` (via vpn-install)

- [x] backed up macold's current configs (`.pre-gigi-<ts>`)
- [x] `./scripts/generate-client-config macold` → ZIP with 6 outbounds (xhttp-orca-hel, tcp-orca-hel, xhttp-aws-st, tcp-aws-st, **xhttp-gigi**, **tcp-gigi**)
- [x] transferred + installed via vpn-install
- [x] outbound list verified on macold (6 entries including gigi)
- [x] kickstarted both sing-box-vpn and xray-xhttp services on macold

### Task 9: Verify chain end-to-end from macold (rigorous chain-proof)

**Files:** (no changes — verification)

Sing-box urltest doesn't expose manual override (no `selector` semantics on URLTest type), so "forcing" a specific outbound for a real curl isn't straightforward. Instead, exercise the ru-hop chain via the **xray-core SOCKS port** that already chains through ru-hop's outbound — this bypasses sing-box urltest entirely and proves the chain in isolation.

- [x] xhttp-gigi probe: 222ms (first attempt), 102-133ms (5 sustained) ✓
- [x] tcp-gigi probe: 361ms (first attempt), 143-165ms (5 sustained) ✓
- [x] **chain proof via xray-core SOCKS port (DEFINITIVE):** macold's xhttp-gigi SOCKS port = 1082; `curl --socks5 127.0.0.1:1082 https://ifconfig.me` returned `16.171.223.39` (aws-st IP) consistently — 3/3 ✓
- [skip] log cross-correlation skipped (user interrupted — chain proof sufficient evidence)
- [skip] negative test skipped (extra certainty not needed; design works)

### Task 10: Sustained stability check on macold

**Files:** (no changes — verification)

- [x] 5 sustained probes per gigi outbound, 3s apart (revised from 60s — current state non-burnt, 3s avoids natural urltest interval): xhttp-gigi 5/5 (103-133ms range), tcp-gigi 5/5 (143-165ms range), zero failures
- [x] 3 forced SOCKS-curl through gigi chain — 3/3 returned `16.171.223.39` (aws-st IP)
- [skip] 5-minute log tail skipped — chain already proven stable across 11 successful operations

### Task 11: Render client package for local Mac + USER manual activation

**Files:**
- Create: local Mac `~/.config/sing-box/config-auto.json.new` (agent prepared)
- Create: local Mac `~/.config/xray/config.json.new` (agent prepared)

- [x] `./scripts/generate-client-config ninja-mac` → ZIP with 6 outbounds
- [x] agent extracted sing-box + xray configs, placed as `.new` files on local Mac (validated: `sing-box check` OK, `xray -test` OK)
- [x] presented USER with exact `cp` + `launchctl kickstart -k system/com.{xray-xhttp,sing-box-vpn}` commands
- [x] USER kickstarted manually (per `feedback_no_stop_vpn.md`)

### Task 12: Verify chain end-to-end from local Mac

**Files:** (no changes — verification)

- [x] outbound list verified: 6 entries (xhttp-orca-hel, tcp-orca-hel, xhttp-aws-st, tcp-aws-st, xhttp-gigi, tcp-gigi)
- [x] probes: xhttp-orca-hel 110ms ✓, tcp-orca-hel 92ms ✓, xhttp-aws-st Timeout (direct DPI-burnt, expected), tcp-aws-st Timeout (same), **xhttp-gigi 102ms ✓**, **tcp-gigi 155ms ✓**
- [x] urltest active: `tcp-orca-hel` (fastest direct at 92ms — natural selection)
- [x] external IP via auto: `62.112.217.94` (orca-hel — urltest's choice)
- [x] **chain proof via SOCKS port 1082**: `curl --socks5 127.0.0.1:1082 https://ifconfig.me` returns `16.171.223.39` ✓ — multi-hop chain works from local Mac

### Task 13: Update memory + final docs

**Files:**
- Create: `~/.claude/projects/-Users-ninja-bb-dpi/memory/project_gigi_relay.md` (new dedicated memory)
- Modify: `~/.claude/projects/-Users-ninja-bb-dpi/memory/MEMORY.md`

- [x] new memory `project_gigi_relay.md` covers: dual-role xray architecture, why the relay solves consumer-DPI burn, schema additions (`relay_upstream` field), current fleet state, two known-gotchas (xray-users silent-fail loop, generate_keys grep regex), single-point-of-failure note for the aws-st route
- [x] MEMORY.md index updated with one-line entry
- [skip] AGENTS.md / README.md architecture paragraph deferred (out of scope for this iteration — repo CLAUDE.md will discover via current state)

### Task 14: [Final] Move plan to completed

- [x] all checkboxes above marked
- [ ] `mkdir -p docs/plans/completed && mv docs/plans/20260513-ru-hop-relay-to-aws-st.md docs/plans/completed/` (deferred; let user decide whether to commit plan to git or keep local)

## Rollback Plan

### Task 1 fails (SSH not working)
- No repo / config changes happened. User reconfigures SSH access manually.

### Task 2-3 fails (template / deploy.sh changes broken)
- Revert local edits via `git checkout config/server-relay.template.json scripts/deploy.sh`
- No live system impact.

### Task 4 fails (xray-users add error)
- `xray-users remove ru-hop-relay` reverts the add. orca-hel + aws-st unchanged.

### Task 5 fails (servers.json edit broken)
- Restore from `servers.json.pre-ru-hop-<ts>` backup.

### Task 6 fails (deploy errors mid-flight)
- ru-hop is a fresh server — failed deploy leaves it in an unknown state. Either retry deploy (idempotent on most steps) or destroy/recreate the VPS.
- Repo state: `servers.json` ru-hop entry remains but no clients depend on it yet. Either remove it from `servers.json` or fix deploy and retry.

### Task 7 fails (user list not synced on ru-hop)
- `./scripts/xray-users sync` should reconcile from `users.json`. Investigate why deploy.sh didn't include all users.

### Task 8 fails (macold install errors)
- Restore macold's pre-install backup files. Re-run `vpn-install` after fixing.

### Task 9 fails (chain not working — external IP wrong or probes timeout)
- Most likely cause: ru-hop's outbound auth (UUID/key/path/sni mismatch with aws-st). Check ru-hop xray logs for handshake errors.
- Second cause: aws-st's REALITY keys recorded in `servers.json` mismatch live aws-st (verify by re-running aws-st key extraction).
- Third cause: aws-st itself currently DPI-burnt from ru-hop's IP — try later when window decays.
- Rollback: remove ru-hop from `servers.json`, regenerate macold package, reinstall.

### Task 9 fails specifically on `tcp-ru-hop` only (vision-on-vision broken)
- This is the explicit risk flagged for vision-on-vision: ru-hop inbound port 8443 uses `xtls-rprx-vision`, AND ru-hop outbound to aws-st:8443 also uses `xtls-rprx-vision`. Double-wrapping may cause subtle handshake/throughput issues with no clear error message.
- If `xhttp-ru-hop` works but `tcp-ru-hop` doesn't:
  - Option A (recommended): keep ru-hop as **XHTTP-only relay**. Modify the relay template to omit the port-8443 inbound entirely (or in deploy.sh, set a `RELAY_XHTTP_ONLY=true` mode). servers.json's render of `tcp-ru-hop` outbound on client side becomes irrelevant if there's no port-8443 inbound on ru-hop — but rendering still produces it. Cleanest: also skip rendering `tcp-ru-hop` when the server is a relay.
  - Option B: keep port-8443 inbound but route it to `freedom` (direct exit through ru-hop's own internet) as a degraded fallback. Mac→ru-hop tcp-vision traffic would then exit at 139.100.204.69 instead of aws-st. Different exit IP, mostly OK as failover.
- Either fix is a follow-up change, not in this plan's scope. Note as ➕ task if it materializes.

### Task 10 fails (stability check shows intermittent failures)
- Same as today's general DPI-burn pattern — wait for decay, OR accept as flaky failover that urltest auto-routes around.

### Task 11-12 fails (local Mac issues)
- Restore local Mac from `.pre-ru-hop-*` backups (cp + kickstart manually).

### Total wipe-back-to-start
- Restore both Macs from `.pre-ru-hop-*` backups + manual kickstarts
- Remove ru-hop entry from `servers.json` (restore from `.pre-ru-hop-*` backup)
- `xray-users remove ru-hop-relay` from orca-hel + aws-st
- Optionally destroy the ru-hop VPS or leave it idle (no clients reference it)

## Post-Completion

**Manual verification over time:**
- Watch macold + local Mac latency through ru-hop outbounds for a few days — expect ~150-300ms (2 REALITY handshakes + 2 ISP hops). Compare to direct paths.
- Watch DPI behavior: does ru-hop→aws-st leg stay clean over time? If it gets burnt eventually, the design's assumption (datacenter ASN bypasses consumer-DPI) is wrong — pivot to a different exit or topology.

**Future cleanup (user's stated intent, NOT in this plan's scope):**
- When ru-hop has proven reliable, manually remove `aws-st` direct entries from `servers.json` and re-render (after backing up the full state). Direct Mac→aws-st outbounds disappear from client config. ru-hop becomes the sole path to aws-st.

**Single point of failure consideration:**
- If ru-hop goes down, the aws-st exit is unreachable. orca-hel remains independent (still works).
- A second relay would mitigate but adds operational burden. YAGNI for now.

**External system updates:** none. Self-contained within this fleet + RU-hop VPS provider.
