# Snapshot — what we know about RU consumer ISP DPI as of 2026-05

A cross-source synthesis of observed and inferred RU consumer ISP DPI
behavior, drawn from this project's first-hand fleet-debugging
experience and the documented patterns in the source pages.

## TL;DR

The censor doesn't statically fingerprint REALITY. It does several
*different* things, each of which alone is solvable but which compound
when stacked. Specifically:

1. **SNI/IP correlation drops** keyed on (SNI, destination ASN)
   tuples. Solved by [[asn-match-sni-camouflage]].
2. **Time-windowed flow-learning** triggered by sustained probe-
   failure bursts. Solved by minimising probe-failure surface (single-
   server pool, no unnecessary kickstarts, hide upstream-only servers
   via `client_render: false`). See [[dpi-flow-learning]].
3. **Transport-shape preference**: TCP+vision is more often burnt than
   XHTTP on the same server. XHTTP's HTTP-2-like framing + padding
   blends better. See [[xhttp-transport]].
4. **Unconfirmed-but-plausible**: JA3/JA4 fingerprint analysis,
   latency-delta active probing, DNS↔TLS cross-flow correlation. Not
   currently observed as deployed against this project's fleet but
   defensible from public research. See [[utls-fingerprint-staleness]],
   [[latency-delta-active-probe]], [[doh-for-split-routed-dns]].

## What's actually been seen

| Behavior | Evidence | Source |
|---|---|---|
| (SNI on a non-matching ASN, dest_ASN=cloud) payload drop | Two-sided tcpdump, May 2026 | [[s-memory-sni-asn-correlation-incident]] |
| Time-windowed flow burn (1-3h decay) | Same incident; sandbox-inbound A/B | [[s-memory-sni-asn-correlation-incident]] |
| XHTTP/443 survives where TCP+vision/8443 is dropped | Same incident, same server | [[s-memory-sni-asn-correlation-incident]] |
| Provider-family SNI on a non-cloud exit causes unpredictable wedges | 2026-05-13, multi-hour rollback | [[s-memory-sni-asn-correlation-incident]] line 41+ |
| Active probing of REALITY endpoints | Not directly observed; inferred from REALITY's design rationale | — |

The "active probing" entry is worth noting: REALITY is designed to
defeat active probing, but this project has not directly captured an
active-probe attempt against its fleet IPs. The DPI behaviors we *have*
observed are passive-correlation + flow-learning, both of which work
without active probing.

## What works against the censor today

The project's current shipped configuration is the empirical answer
to "what works on consumer ISP egress":

1. **REALITY+XHTTP+REALITY chain** through a same-region datacenter
   relay to a cloud-region exit. The relay's first-hop ASN-match
   gives in-region camouflage; the relay→cloud leg moves the cross-
   border traffic onto a datacenter-AS profile that isn't on the DPI
   hot-list. See [[s-memory-chain-relay-rationale]].
2. **`client_render: false`** on upstream-only servers, so clients
   only ever probe the relay directly. Prevents probe-failure spam
   against the cloud-region exit. See [[s-arch-decisions]] §2.8.
3. **`--proto xhttp`** *recommended* for RU-egress chain clients —
   passed explicitly at the call site to drop TCP+vision from the
   client urltest pool. The global default in `scripts/render-config`
   stays `all` (preserves TCP+vision as a render-time escape hatch
   for non-RU paths). Server keeps the TCP+vision inbound listening
   regardless.
4. **TFO=false on the TCP+vision outbound** ([[s-arch-decisions]] §1.6).
   Even though the active client pool doesn't use TCP+vision, the
   outbound is rendered if `--proto all` is ever used, and the TFO
   signal would be inconsistent with the uTLS-chrome impersonation.
5. **DoH for split-routed regional DNS** (PR #12). Closes the cross-
   flow DNS↔TLS correlation surface.

## What we expect to need next

In order of likelihood (and cheapness to mitigate):

1. **JA4 staleness** is the most-likely next signal the censor will
   exploit at scale. Mitigation: auto-update sing-box + xray binaries
   regularly. See [[utls-fingerprint-staleness]].
2. **urltest probe interval jitter** — render `interval` as a
   `random(60s, 120s)` per-client at install time to break
   synchronized-wave probe patterns. Cheap.
3. **Probe-failure backoff supervisor** — userspace watchdog over
   sing-box's log that pauses the urltest pool after N consecutive
   failures. Defends against [[dpi-flow-learning]] under transient
   network issues.
4. **REALITY private_key / xhttp_path / short_id rotation**. Currently
   no automated rotation. Operational cost: high. Acceptable until a
   REALITY auth-bypass disclosure ever lands.

## Open research questions

- **Is JA4 fingerprint analysis actually deployed against this
  project's fleet today, or only theoretically?** Would need to capture
  the wire ClientHello + compute JA4 from a known-burnt outbound and
  compare to real-Chrome JA4 from the same client IP.
- **What's the actual decay window for the flow-block?** Documented
  rough range is 1-3 hours; tighter measurement would help size
  retry-cooldown durations.
- **Is the time-windowed flow-learning per-flow-tuple or per-(IP, SNI,
  port)?** Sandbox-inbound A/B suggests per-tuple, but more replicas
  would harden the claim.

## Sources

- [[s-arch-decisions]]
- [[s-memory-sni-asn-correlation-incident]]
- [[s-memory-chain-relay-rationale]]
- [[s-memory-twosided-tcpdump]]

## Last updated

2026-05-14 — wiki bootstrap. Update on every ingest of a relevant
new source.
