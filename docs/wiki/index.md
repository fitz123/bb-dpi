# Wiki index

Catalog of every page in this wiki. Organized by category. Each entry:
title, one-line summary, supporting sources.

When ingesting a new source, update this file to reflect any new pages
created or modified.

## Concepts

| Page | Summary | Sources |
|---|---|---|
| [[reality-protocol]] | XTLS's anti-active-probe TLS-on-the-wire scheme. Forwards probes to a real `dest` site so the server is indistinguishable from a reverse proxy. | [[s-memory-chain-relay-rationale]], [[s-2026-05-tspu-asn-camouflage-research]], [[s-2026-05-xray-relay-community-reports]], xray-core source (`MirrorConn`) |
| [[asn-match-sni-camouflage]] | Strategy: REALITY `dest` hostname must resolve to the same AS as the server's own IP. Defeats coarse SNI/IP correlation attacks. | [[s-memory-sni-asn-correlation-incident]], [[s-memory-chain-relay-rationale]], [[s-2026-05-tspu-asn-camouflage-research]], [[s-2026-05-xray-relay-community-reports]], [[s-tool-rkn-block-checker]] |
| [[dpi-flow-learning]] | RU DPI builds time-windowed flow-blocks triggered by sustained probe-failure bursts. Decays in 1-3 hours. Adjacent: 15-20 KB freeze, CIDR whitelist, port-bias, empty-SNI exemption, IPv6 parity. | [[s-memory-sni-asn-correlation-incident]], [[s-memory-twosided-tcpdump]], [[s-2026-05-tspu-asn-camouflage-research]], [[s-2026-05-ipv6-bgp-path-aws-stockholm]], [[s-tool-rkn-block-checker]] |
| [[xhttp-transport]] | xray-core's HTTP/2-like REALITY transport. Web-traffic-shaped wire signature; more resilient on RU consumer DPI than TCP+vision. | [[s-memory-sni-asn-correlation-incident]], [[s-memory-chain-relay-rationale]], Xray XHTTP discussion #4113 |
| [[two-sided-tcpdump-diagnostic]] | Diagnostic technique for the "dead tunnel" failure class — capture on both endpoints simultaneously, compare PSH patterns. | [[s-memory-twosided-tcpdump]], [[s-tool-rkn-block-checker]] |
| [[utls-fingerprint-staleness]] | uTLS's frozen Chrome ClientHello lags real Chrome releases by weeks-months. Plausible discriminator if censor builds a JA3/JA4 list. | xray-core source, sing-box docs |
| [[hosting-provider-as-dpi-variable]] | Provider choice is a DPI-evasion variable, not just procurement. Three axes: source-ASN ↔ SNI pairing, compliance posture, TSPU inspection profile of the source ASN. | [[s-2026-05-ru-vps-ipv6-procurement-scan]], [[s-2026-05-tspu-asn-camouflage-research]], [[s-2026-05-xray-relay-community-reports]], [[s-2026-05-ipv6-bgp-path-aws-stockholm]] |

## Sources

| Page | Type | Subject | Ingested |
|---|---|---|---|
| [[s-memory-sni-asn-correlation-incident]] | out-of-repo memory | 2026-05-13 incident on cloud-region exit — SNI/IP correlation drop + time-windowed flow-learning | 2026-05-14 |
| [[s-memory-chain-relay-rationale]] | out-of-repo memory | VLESS chain-relay rationale, ASN-match SNI on same-region datacenter | 2026-05-14 |
| [[s-memory-twosided-tcpdump]] | out-of-repo memory | Diagnostic technique origin + operational constraints | 2026-05-14 |
| [[s-2026-05-ru-vps-ipv6-procurement-scan]] | external web research | May 2026 RU VPS market scan: who ships native dual-stack v4+v6 by default, who's disqualified, top 3 picks | 2026-05-16 |
| [[s-2026-05-ipv6-bgp-path-aws-stockholm]] | external research (bgp.tools, RIPE Stat) | IPv6 BGP path quality from RU hosters to AWS eu-north-1; v4-vs-v6 path divergence; reported TSPU v6 parity claim (single-source, see #evidence-grade) | 2026-05-16 |
| [[s-2026-05-xray-relay-community-reports]] | external web research (forums, GitHub, news) | 2025-2026 community consensus on RU chain-relay providers; dated DPI events timeline; v6-as-bypass weak-evidence assessment | 2026-05-16 |
| [[s-2026-05-tspu-asn-camouflage-research]] | external research, measurement-weighted on architecture / community-hypothesised on named mechanisms | TSPU fingerprinting techniques 2025-2026; per-provider ASN inspection profile; community-hypothesised SNI/IP correspondence mechanism for first-party-observed drop; provider compliance posture | 2026-05-16 |
| [[s-tool-rkn-block-checker]] | external community tool (CLI, MIT, on PyPI) | First-triage TSPU diagnostic that classifies failures per-layer (DNS/TCP/TLS/HTTP). Documented signal patterns map to existing wiki concepts. README's "v6 less filtered" claim nuances the single-source v6-parity claim. | 2026-05-17 |

## Synthesis

| Page | Summary | Last updated |
|---|---|---|
| [[2026-05-ru-dpi-snapshot]] | Cross-source synthesis: observed RU consumer ISP DPI behavior as of 2026-05, what works today, what we expect to need next. Adds 2025-2026 public-research findings (15-20 KB freeze, CIDR whitelist, port-bias, AI-keyed REALITY detector, TSPU v6 parity, mobile whitelist) and the hosting-provider-as-DPI-variable framework. | 2026-05-16 |
| [[dns-aaaa-cascade-failure]] | Methodology teaching case for a single-observation investigation. The mechanism hypothesis (TSPU AAAA-drop on 8.8.8.8) is most-likely invalidated by a post-hoc routing check showing sing-box auto-route was intercepting all system DNS — *conditional on continuous TUN auto-route through the observation window*. Not a citable mechanism. See page Status section. | 2026-05-17 |

## Concepts referenced but not yet a page

These are mentioned in other pages but don't have a dedicated concept
page yet. Candidates for ingestion as the wiki grows:

- `latency-delta-active-probe` — REALITY's structural limit; probes
  to the REALITY server add the dest-RTT vs. probes direct to dest.
  *Referenced in:* [[reality-protocol]], [[asn-match-sni-camouflage]],
  [[xhttp-transport]].
- `doh-for-split-routed-dns` — DoH mitigation for DNS↔TLS cross-flow
  correlation. *Referenced in:* [[reality-protocol]],
  [[asn-match-sni-camouflage]], [[2026-05-ru-dpi-snapshot]].
- `reality-key-rotation-gap` — no automated rotation lifecycle for
  REALITY private keys / shortIds / xhttp_path. *Referenced in:*
  [[reality-protocol]].
- `tcp-option-coherence` — TCP-layer fingerprint coherence with the
  uTLS-chrome impersonation (TFO, MSS, Window Scale, etc.).
  *Referenced in:* [[reality-protocol]].
- `empirical-sni-failures` — case studies of strict-ASN-match SNI
  attempts that backfired. *Referenced in:*
  [[asn-match-sni-camouflage]].

When one of these gets a second reference (or someone explicitly asks),
promote it to a dedicated `concepts/` page.
