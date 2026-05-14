# ADR-001: Remove tcp_multi_path from outbounds

## Status

Accepted (2026-04-09). **Partially superseded 2026-05-14** — the
`tcp_fast_open: true` clause of the original Decision was reversed
(see "Update" section below). The `tcp_multi_path` removal still
stands.

## Context

sing-box outbounds support `tcp_multi_path: true` which enables Multipath TCP (MPTCP) — a TCP extension that allows a single connection to use multiple network paths simultaneously. In theory this improves reliability and throughput.

We enabled `tcp_multi_path` on all VLESS TCP+vision outbounds to improve connection resilience.

## Problem

On macOS (macold, ARM64, Darwin 24.6.0), MPTCP caused persistent `dial tcp <server>:8443: invalid argument` errors on VLESS outbound connections. The connection dial phase succeeded (urltest health checks passed), but actual data transfer failed with the syscall-level error.

This created a failure mode where:

1. urltest probes **passed** (TCP connect succeeded)
2. urltest selected the outbound as healthy
3. Real traffic through the outbound **failed** with `invalid argument`
4. All DNS and traffic routed to the "healthy" but broken outbound
5. Complete connectivity loss until urltest happened to pick the other server

The error persisted across restarts and affected both servers. Removing `tcp_multi_path` immediately resolved the issue.

## Decision

Remove `tcp_multi_path: true` from all VLESS outbounds in `render-config`. Keep `tcp_fast_open: true` which works reliably.

## Consequences

- MPTCP benefits (multi-path resilience) are lost — acceptable since urltest already provides failover across multiple servers and transports
- Eliminates a class of silent failures where urltest thinks an outbound is healthy but data doesn't flow
- One less platform-dependent feature to debug

## Update (2026-05-14): `tcp_fast_open` default flipped to `false`

The original Decision said "Keep tcp_fast_open: true which works reliably."
That stance is reversed for DPI-evasion reasons:

- Real Chrome / Safari / Firefox on macOS do NOT enable TFO by default;
  Chromium dropped it on macOS years ago due to middlebox interop issues
- TFO=true on the outbound emits `TCP option kind 34 (Fast Open Cookie Request)`
  on every fresh SYN — a TCP-layer fingerprint that contradicts the uTLS
  Chrome impersonation at the TLS layer; macOS apps that emit TFO outbound
  to non-Apple IPs are dominated by VPN/proxy clients, so this is a
  near-zero-false-positive proxy discriminator
- Project memory `feedback_twosided_tcpdump_dead_tunnel.md` already named
  TFO `cookiereq` as "a common trigger" worth disabling on SYN-ACK-retransmit
  failure debugging
- The original "works reliably" claim was about local reliability, not DPI
  observability — both can be true; the trade-off was re-prioritized after
  a dialectic + dual-review analysis on 2026-05-14

Performance impact (~30-100ms slower probe init from losing the TFO 1-RTT
savings) is invisible against the multi-RTT REALITY+vision handshake.
Validated end-to-end via test-client redeploy with `--proto all`: sing-box
urltest selected the `tcp-*` outbound (TFO=false) as active and successfully
relayed traffic at 50-70ms handshakes to multiple destinations.
