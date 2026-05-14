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
  outbounds. Trade-off accepted in [[s-arch-decisions]] §7.6.
- **Avoid xray service kickstart**: each kickstart causes urltest
  probe failures across all `xhttp-*` outbounds during the restart
  window. Operational rule: don't kickstart xray unless necessary.
- **Test client validates first**: any deploy that *might* cause
  probe failures gets validated on a test-vantage client first
  (see [[s-arch-decisions]] §6.2). Burning the test client is
  recoverable; burning the operator's primary client mid-session
  isn't.

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

## Sources

- [[s-memory-sni-asn-correlation-incident]] — primary source for the observed
  pattern, including the 2026-05-13 incident where a cloud-region exit's SNI
  swap triggered urltest probe failures that burnt a separate
  healthy outbound for hours.
- [[s-memory-twosided-tcpdump]] — the diagnostic technique that
  surfaced this class of failure (TCP up, payload dropped).
- [[s-arch-decisions]] §7.5 (urltest cadence as DPI trigger), §C1
  (operational rule against unnecessary kickstarts).
