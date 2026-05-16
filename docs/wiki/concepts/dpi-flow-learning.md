# Time-windowed DPI flow-learning

A documented adversary technique observed in RU consumer ISP DPI in
2026: the DPI does *not* permanently fingerprint an SNI/IP pair as
proxy. Instead, it builds **time-windowed flow-blocks** triggered by
*sustained probe-failure patterns* against a specific 5-tuple. The
block decays over hours.

This shapes a lot of operational behavior in [[reality-protocol]]
deployments — most of the DPI-fragility this project has hit is
flow-learning, not static fingerprinting.

## Observed pattern

- The censor's middlebox sees a TLS 5-tuple
  `(client_ip, client_port, server_ip, 443, TCP)` carrying repeated
  TLS-like handshakes that *fail* (do not transition to steady-state
  bidirectional payload data).
- After ~5-10 failures in quick succession (seconds to minutes
  window), the middlebox starts dropping application-data PSH packets
  on that flow. TCP control packets (SYN/ACK/FIN) traverse normally.
- The drop persists for some hours (decay window measured empirically
  in the 1-3 hour range), then traffic on the same flow starts
  working again with no operator action.
- The trigger is **the probe-failure pattern**, not the SNI value or
  the server IP. Same SNI/IP on a sandboxed inbound (different
  client/different port, no failure history) keeps working while the
  burnt flow is blocked.

## Why this matters for client urltest design

The standard auto-failover pattern (sing-box `urltest`) probes each
outbound on a fixed interval (default 30s) and selects the lowest-
latency healthy one. When the network is normal, probes succeed and
the pattern is invisible.

But when *anything* causes a transient outage on one outbound — a
service restart, a brief upstream issue, packet loss spike, REALITY
clock skew, a brief upstream-chain outage — urltest enters a
**probe-failure loop** at 30-second cadence against the failing
outbound. After 5-10 failures (2.5-5 minutes of failures), the
flow-learning heuristic fires. The flow is now burnt.

Worst case: a *transient* upstream issue (which would self-heal in
seconds) gets converted into an *hours-long* outage because the
client's probe machinery triggered the censor's learning during the
brief window.

This was empirically observed (see [[s-memory-sni-asn-correlation-incident]]): a
service kickstart during a config swap triggered urltest probe
failures on a healthy outbound, which then got burnt for the rest of
the day.

## Mitigations available today

- **Single-server urltest pool**: with `client_render: false` hiding
  upstream-only servers, the operator's RU-vantage client pool has
  exactly one outbound. Probe failures against that one outbound have
  nowhere to fail over to, but they also can't burn *other*
  outbounds. Acceptable trade-off given the empirical cost of
  cross-outbound flow burns.
- **Avoid xray service kickstart**: each kickstart causes urltest
  probe failures across all `xhttp-*` outbounds during the restart
  window. Operational rule: don't kickstart xray unless necessary.
- **Test client validates first**: any deploy that *might* cause
  probe failures gets validated on a test-vantage client first.
  Burning the test client is recoverable; burning the operator's
  primary client mid-session isn't.

## Mitigations NOT YET implemented (TODOs)

- **Probe-failure backoff supervisor**: a userspace watchdog that
  parses sing-box's log, detects ≥N consecutive probe failures on
  one outbound, and re-renders the urltest pool *without* that
  outbound for a 30-minute cooldown. sing-box's urltest has no
  native backoff knob, so this lives outside sing-box.
- **Probe interval jitter**: rendering interval as `random(60s,
  120s)` per-client at install time would break the synchronized-
  wave pattern across multiple clients. Cheap but not done.

## Diagnostic signature

A flow burn manifests as a **dead tunnel**: TCP handshake completes,
the client thinks the outbound is healthy ("alive" by urltest
criterion), but real payload traffic fails. The signature is unique
to in-path payload-drop and is the canonical use case for
[[two-sided-tcpdump-diagnostic]].

## Related TSPU heuristics (not pure flow-learning, but adjacent)

Public research surfaces several other RU DPI behaviors that interact
with the flow-learning model. They're documented elsewhere; mention
here to clarify what flow-learning is NOT:

- **15-20 KB foreign-IP freeze** (Jun 2025+): TLS-1.3 connections to
  non-whitelisted foreign DCs (Hetzner, DO, OVH, Cloudflare, AWS)
  silently choke once past ~15-20 KB. Distinct from flow-learning:
  triggered by destination + payload-size, not by probe-failure
  history. See [[s-2026-05-tspu-asn-camouflage-research]].
- **CIDR whitelist on destination** (late 2025): TSPU pre-filters
  by destination CIDR. Distinct from flow-learning: static, not
  trigger-based. See [[s-2026-05-xray-relay-community-reports]].
- **Port 443 selection-bias**: identical VLESS+REALITY on 443 hits
  TSPU inspection at much higher rate than on high ports (47000+).
  Distinct from flow-learning: static port-based sampling.
- **Empty SNI exemption**: empty-SNI flows are not inspected
  enough to trigger any of the above 100% of the time. Confirms
  TSPU's filter is SNI-presence + heuristic, not pure blocklist.
- **TSPU IPv6 inspection parity** (Mar 2026, *single-source claim*):
  IPv6 traffic is reportedly inspected on equal footing with IPv4 —
  if this holds, v6 is not a flow-learning bypass. See
  [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]] for the
  corroboration-gap note.

## Sources

- [[s-memory-sni-asn-correlation-incident]] — primary source for the observed
  pattern, including the 2026-05-13 incident where a cloud-region exit's SNI
  swap triggered urltest probe failures that burnt a separate
  healthy outbound for hours.
- [[s-memory-twosided-tcpdump]] — the diagnostic technique that
  surfaced this class of failure (TCP up, payload dropped).
- [[s-2026-05-tspu-asn-camouflage-research]] — adjacent TSPU
  heuristics (15-20 KB freeze, CIDR whitelist, port-bias, empty-SNI)
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]] — TSPU v6 parity (Mar 2026)
