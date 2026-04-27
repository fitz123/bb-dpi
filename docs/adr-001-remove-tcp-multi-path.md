# ADR-001: Remove tcp_multi_path from outbounds

## Status

Accepted (2026-04-09)

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
