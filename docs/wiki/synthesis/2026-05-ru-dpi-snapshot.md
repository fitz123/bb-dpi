# Snapshot — what we know about RU consumer ISP DPI as of 2026-05

A cross-source synthesis of observed and inferred RU consumer ISP DPI
behavior, drawn from this project's first-hand fleet-debugging
experience and the documented patterns in the source pages.

## TL;DR

The censor doesn't statically fingerprint REALITY. It does several
*different* things, each of which alone is solvable but which compound
when stacked. Specifically:

1. **SNI/IP correspondence drops** keyed on the destination IP's ASN
   vs the SNI's authoritative DNS-answer ASN. Community-hypothesised
   mechanism (Habr QnA), first-party measured *effect* — the
   operator's "generic-CDN-SNI on a cloud-region exit" drop incident
   is the measured fingerprint. Solved (empirically) by
   [[asn-match-sni-camouflage]], whose operational guidance holds
   regardless of which specific mechanism produces the drop.
2. **CIDR whitelist on destination** (late 2025): TSPU pre-filters
   by destination CIDR — explains why foreign-cloud destinations
   (AWS, Hetzner, DO, OVH, Cloudflare) burn fast. See
   [[s-2026-05-tspu-asn-camouflage-research]].
3. **15-20 KB foreign-IP freeze** (Jun 2025+): TLS-1.3 to
   non-whitelisted foreign DCs silently chokes past ~15-20 KB. This
   is the structural reason the chain-relay design is necessary:
   the chain's foreign-leg is server-to-server within RU peering,
   bypassing the consumer-egress filter.
4. **Time-windowed flow-learning** triggered by sustained probe-
   failure bursts. Solved by minimising probe-failure surface (single-
   server pool, no unnecessary kickstarts, hide upstream-only servers
   via `client_render: false`). See [[dpi-flow-learning]].
5. **Port 443 selection-bias**: identical VLESS+REALITY on 443 hits
   inspection at near-100% rate; on 47000+ ~80% passes. REALITY's
   port-443 default now means maximum inspection sampling.
6. **Transport-shape preference**: TCP+vision is more often burnt than
   XHTTP on the same server. XHTTP's HTTP-2-like framing + padding
   blends better. See [[xhttp-transport]].
7. **AI-keyed REALITY detector** (Dec 2025, *single-source claim*):
   RKN reportedly deployed an ML detector on TLS-1.3 to
   non-whitelisted foreign IPs. xray-core 25.12.8+ adds `testpre` /
   `testseed` defenses. See
   [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]] for the
   corroboration-gap note.
8. **IPv6 inspection parity** (Mar 2026, *single-source claim*):
   TSPU is reported to inspect v6 on equal footing with v4. If this
   holds, v6-as-bypass is no longer load-bearing. See
   [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]] — claim
   is single-sourced to an LLM-generated docs site, pending
   independent corroboration.
9. **Unconfirmed-but-plausible**: JA3/JA4 fingerprint analysis,
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

## Public research observed (not yet measured first-hand)

These are documented in public sources but not yet first-party
confirmed against the operator's fleet. Adding them here so the
synthesis tracks the broader threat model, not just direct
observations.

| Behavior | Date | Source |
|---|---|---|
| 15-20 KB freeze on TLS-1.3 to foreign DCs | Jun 2025+ | [[s-2026-05-tspu-asn-camouflage-research]] |
| Destination-CIDR whitelist pre-filter | Late 2025 | [[s-2026-05-tspu-asn-camouflage-research]] |
| Port 443 vs high-port selection-bias | 2025 | [[s-2026-05-tspu-asn-camouflage-research]] |
| Empty-SNI flows skip all heuristics | 2025 | [[s-2026-05-tspu-asn-camouflage-research]] |
| ECH blocked | Nov 2024 | OONI |
| AI-keyed REALITY detector on TLS-1.3 to non-whitelist foreign IPs (reported, single-source) | Dec 2025 | [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]] |
| TSPU IPv6 inspection parity (reported, single-source) | Mar 2026 | [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]] |
| Mobile-whitelist denial of VPN clients on Gosuslugi/Sberbank/Yandex | Apr 2026 | [[s-2026-05-xray-relay-community-reports]] |

## Hosting-provider choice as a DPI-evasion variable

A 2026-05 procurement-side research wave surfaced
[[hosting-provider-as-dpi-variable]] as a load-bearing concept: the
relay's hosting provider materially shapes the camouflage primitives
available, the relay's longevity bound, and the TSPU inspection
posture against the source ASN. Three axes (source-ASN ↔ SNI pairing,
compliance posture, inspection profile) interact in tension; see
the concept page for the framework.

The procurement decision underlying the v6-relay test
([[s-2026-05-ru-vps-ipv6-procurement-scan]]) was originally framed
as "v6 BGP path may bypass v4-centric DPI". Both public-research
sources surfaced in 2026-05 ([[s-2026-05-ipv6-bgp-path-aws-stockholm]],
[[s-2026-05-xray-relay-community-reports]]) found the v6-as-bypass
hypothesis unsupported in 2026. Two independent low-confidence legs:
(a) no measurement-based community evidence of v6 helping
(absence-of-evidence); (b) reported TSPU v6 parity Mar 2026
(single-source, see
[[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]]). The
procurement conclusion stands even if leg (b) is later refuted,
because leg (a) holds on its own. Provider choice should be reframed
around axes 1 and 2 instead.

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
   against the cloud-region exit.
3. **`--proto xhttp`** *recommended* for RU-egress chain clients —
   passed explicitly at the call site to drop TCP+vision from the
   client urltest pool. The global default in `scripts/render-config`
   stays `all` (preserves TCP+vision as a render-time escape hatch
   for non-RU paths). Server keeps the TCP+vision inbound listening
   regardless.
4. **TFO=false on the TCP+vision outbound**. Even though the active
   client pool doesn't use TCP+vision, the outbound is rendered if
   `--proto all` is ever used, and the TFO signal would be
   inconsistent with the uTLS-chrome impersonation.
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

- [[s-memory-sni-asn-correlation-incident]]
- [[s-memory-chain-relay-rationale]]
- [[s-memory-twosided-tcpdump]]
- [[s-2026-05-ru-vps-ipv6-procurement-scan]]
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]]
- [[s-2026-05-xray-relay-community-reports]]
- [[s-2026-05-tspu-asn-camouflage-research]]

## Last updated

2026-05-16 — ingested four 2026-05 web-research sources on RU VPS
procurement, IPv6 BGP path quality to AWS, community-reported
provider experience, and TSPU fingerprinting + ASN-camouflage.
New concept page [[hosting-provider-as-dpi-variable]] added.
v6-as-DPI-bypass hypothesis flagged as unsupported by 2026 public
research.
