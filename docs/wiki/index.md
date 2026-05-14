# Wiki index

Catalog of every page in this wiki. Organized by category. Each entry:
title, one-line summary, supporting sources.

When ingesting a new source, update this file to reflect any new pages
created or modified.

## Concepts

| Page | Summary | Sources |
|---|---|---|
| [[reality-protocol]] | XTLS's anti-active-probe TLS-on-the-wire scheme. Forwards probes to a real `dest` site so the server is indistinguishable from a reverse proxy. | [[s-arch-decisions]] |
| [[asn-match-sni-camouflage]] | Strategy: REALITY `dest` hostname must resolve to the same AS as the server's own IP. Defeats coarse SNI/IP correlation attacks. | [[s-memory-sni-asn-correlation-incident]], [[s-memory-chain-relay-rationale]] |
| [[dpi-flow-learning]] | RU DPI builds time-windowed flow-blocks triggered by sustained probe-failure bursts. Decays in 1-3 hours. | [[s-memory-sni-asn-correlation-incident]], [[s-memory-twosided-tcpdump]] |
| [[xhttp-transport]] | xray-core's HTTP/2-like REALITY transport. Web-traffic-shaped wire signature; more resilient on RU consumer DPI than TCP+vision. | [[s-arch-decisions]], [[s-memory-sni-asn-correlation-incident]] |
| [[two-sided-tcpdump-diagnostic]] | Diagnostic technique for the "dead tunnel" failure class — capture on both endpoints simultaneously, compare PSH patterns. | [[s-memory-twosided-tcpdump]] |
| [[utls-fingerprint-staleness]] | uTLS's frozen Chrome ClientHello lags real Chrome releases by weeks-months. Plausible discriminator if censor builds a JA3/JA4 list. | [[s-arch-decisions]] |

## Sources

| Page | Type | Subject | Ingested |
|---|---|---|---|
| [[s-arch-decisions]] | in-repo synthesis doc | `docs/architecture-decisions.md` — 92 architectural decisions for the bb-dpi project | 2026-05-14 |
| [[s-memory-sni-asn-correlation-incident]] | out-of-repo memory | 2026-05-13 incident on cloud-region exit — SNI/IP correlation drop + time-windowed flow-learning | 2026-05-14 |
| [[s-memory-chain-relay-rationale]] | out-of-repo memory | VLESS chain-relay rationale, ASN-match SNI on same-region datacenter | 2026-05-14 |
| [[s-memory-twosided-tcpdump]] | out-of-repo memory | Diagnostic technique origin + operational constraints | 2026-05-14 |

## Synthesis

| Page | Summary | Last updated |
|---|---|---|
| [[2026-05-ru-dpi-snapshot]] | Cross-source synthesis: observed RU consumer ISP DPI behavior as of 2026-05, what works today, what we expect to need next. | 2026-05-14 |

## Concepts referenced but not yet a page

These are mentioned in other pages but don't have a dedicated concept
page yet. Candidates for ingestion as the wiki grows:

- `latency-delta-active-probe` — REALITY's structural limit; probes
  to the REALITY server add the dest-RTT vs. probes direct to dest.
- `doh-for-split-routed-dns` — PR #12's mitigation for DNS↔TLS
  cross-flow correlation.
- `reality-key-rotation-gap` — no automated rotation lifecycle for
  REALITY private keys / shortIds / xhttp_path.
- `tcp-option-coherence` — TCP-layer fingerprint coherence with the
  uTLS-chrome impersonation (TFO, MSS, Window Scale, etc.).
- `chain-relay-topology` — when, why, how to deploy a relay-vs-exit
  topology.
- `empirical-sni-failures` — case studies of strict-ASN-match SNI
  attempts that backfired.
- `mptcp-on-darwin-quirk` — ADR-001 subject; not generalisable far
  enough to merit its own page yet.

When one of these gets a second reference, promote it to a concept
page.
