---
tags: [ipv6, bgp, aws, eu-north-1, transit, tspu]
sources: [s-2026-05-ru-vps-ipv6-procurement-scan, s-2026-05-xray-relay-community-reports, s-2026-05-tspu-asn-camouflage-research]
updated: 2026-05-16
---

# Source: IPv6 BGP path from RU hosters to AWS eu-north-1

External BGP-data research compiled May 2026 against the question:
does the IPv6 BGP path from major RU hosting ASNs to AWS Stockholm
diverge from the IPv4 path in a way that bypasses RU state DPI?

- **Type**: external research using bgp.tools, bgp.he.net, RIPE Stat
- **Subject**: AS-path quality, transit narrowness, DPI-bypass
  hypothesis under v6
- **Ingested**: 2026-05-16
- **Companion sources**: [[s-2026-05-ru-vps-ipv6-procurement-scan]],
  [[s-2026-05-xray-relay-community-reports]]

## AWS eu-north-1 v6 footprint

- Origin ASN: **AS16509 Amazon** for all AWS IPv6 prefixes.
- Likely eu-north-1 supernets (both AS16509-originated, both heavily
  routed via AS1299 Arelion in Stockholm):
  - `2a05:d014::/36`
  - `2a05:d016::/36`
- AWS's `ip-ranges.json` does not carry IPv6 supernets per region;
  per-region tagging is at /56-and-smaller subdelegations only.
- Practical probe target: `ipv6.ec2-reachability.amazonaws.com`.

## RU-hoster v6 upstream picture

| ASN | Org | Notable v6 upstreams | v6 IXes |
|---|---|---|---|
| **AS49505** | Selectel (default VDS line) | RETN (9002), Rostelecom (12389), TTK (20485), RASCOM (20764). **No AS1299 v6 peer.** | MSK-IX, GNM-IX (NL) |
| **AS50340** | Selectel-MSK (Cloud Platform) | **Arelion (1299)**, RETN, Rostelecom, TTK, RASCOM, Qrator (197068) | MSK-IX v6, AMS-IX v6, PITER-IX, GNM-IX, GLOBAL-IX (NL) |
| **AS9123** | TimeWeb | **Arelion (1299)**, RETN, Cogent (174), Rostelecom, RASCOM | DE-CIX FRA + AMS-IX v6 per PeeringDB |
| **AS210644** | Aeza Intl (NL/DE) | aurologic (30823), HE (6939), Aeza-RU (216246), GNM (31500) | GNM-IX NL; not a *RU-located* relay endpoint |
| **AS216246** | Aeza Group LLC (RU) | RETN-RU (57304), GNM (31500), Telecom-Birzha (199599) | GNM-IX |
| **AS197695** | REG.RU / RuVDS | RETN (9002), TTK (20485), mostly RU-domestic | Narrow Western IX presence |
| **AS25532** | Mastertel/MASTERHOST | RETN (9002), Transroute (50509), IQWeb (59692) | **PITER-IX FRA/Helsinki/Riga/Tallinn**, CLOUD-IX MSK, MSK-IX |

## v4 vs v6 divergence to AWS eu-north-1

- **v6 to `2a05:d014::/36` / `2a05:d016::/36`** — dominant paths are
  Arelion-centric: `… 1299 16509` (Arelion peer of AWS) and
  `… 6939 16509` (HE direct). Tier-1 mesh also: 3356 → 1299 →
  16509, 2914 → 16509, 174 → 16509. Narrow upstream pool at the
  AWS-edge.
- **v4 to `13.51.0.0/16` (eu-north-1)** — significantly broader
  transit fan-in at the AWS-edge: AS8218, AS6908, AS36924, AS48070,
  AS6894, AS31742, AS35266, AS3257, AS2914, AS6461 all directly peer
  with AS16509.
- **Summary**: v6 paths to eu-north-1 are narrower and Arelion-centric.
  For a RU VPS, v6 path is almost always
  `RU-AS → RETN/TTK/Rostelecom → AS1299 → AS16509` in 4 hops; v4 has
  similar length with more transit variety.

## Direct AWS peering near RU

- AWS has direct v6 presence at:
  - Netnod Stockholm (200G)
  - AMS-IX (`2001:7f8:1::a501:6509:1`, 600G)
  - DE-CIX FRA (`2001:7f8::407d:0:1`, 800G)
  - LINX LON1
- AWS has **no presence** at MSK-IX, SPB-IX, PITER-IX, GNM-IX. No
  RU hoster has a 1-hop v6 path to AWS — there is always at least
  one Tier-1 (typically Arelion AS1299) between them.

## The v6-bypass hypothesis, refuted

This is the load-bearing finding for the procurement decision:

- **TSPU reportedly achieved IPv6 inspection parity in March 2026**
  ([deepwiki TSPU DPI analysis](https://deepwiki.com/rcd27/zapret2-mcp/3.5-tspu-deep-packet-inspection-analysis)).
  Bypass tools that historically defaulted to "skip v6" now require
  explicit `DISABLE_IPV6=0`. The assumption "TSPU is v4-only" no
  longer holds in 2026. **See [[#evidence-grade]] — this claim is
  single-sourced to an LLM-generated docs site and is not
  independently corroborated.**
- **December 2025**: Roskomnadzor reportedly deployed an AI-based
  VLESS+REALITY detector keyed on TLS-1.3 handshake patterns to
  non-whitelisted foreign IPs. The detector applies to both v4 and
  v6. **Same source / same evidence grade as the v6-parity claim.**
- **TSPU sits in-ISP, not in transit**. The DPI middlebox sees the
  packet before it leaves the RU AS. "v6 transit being different"
  doesn't avoid TSPU — the choke point is the first hop, not the
  Tier-1 chain. Provider's first-hop ASN matters, transit ASN
  diversity does not. (This architectural fact is corroborated by
  the TSPU IMC22 paper and net4people #490 — independent of the
  v6-parity claim above.)

Implication: switching a chain-leg from v4 to v6 should NOT be
expected to bypass DPI on its own. The v6 path quality (latency,
stability) is still a valid optimization variable; the v6 path as
*DPI evasion* is not.

### Evidence grade

The two 2026-dated claims in this section (TSPU IPv6 inspection
parity Mar 2026; RKN AI-keyed REALITY detector Dec 2025) come from
a single citation: `deepwiki.com/rcd27/zapret2-mcp/3.5-...`.
`deepwiki.com` is Devin AI's auto-generated documentation site for
GitHub repos. It is a *secondary* knowledge base; the underlying
primary content is the `rcd27/zapret2-mcp` repository's own notes
on TSPU behavior. As of the 2026-05 ingest:

- No OONI report corroborates Mar 2026 IPv6 parity.
- No academic / measurement paper corroborates Dec 2025 AI-keyed
  detector.
- No net4people/bbs thread directly confirms either claim.
- The companion source [[s-2026-05-tspu-asn-camouflage-research]]
  explicitly notes "No measurement-grade study of v6 vs v4 by TSPU
  exists in the public corpus reviewed."

Treat both as **single-source community claims, pending
corroboration**. The "reportedly" hedge is load-bearing across
downstream concept and synthesis pages — do not strip it without
adding an independent measurement-based source.

This is exactly the failure mode the Karpathy-gist wiki pattern
warns about (link liberally; flag uncertainty explicitly; prefer
measurement over operator claim). Surfacing it here so the
hypothesis-invalidating finding can be re-evaluated when a primary
source lands.

## Ranking of candidate RU ASNs for v6 transit *quality* to AWS-Stockholm

If procuring on path-quality grounds alone (DPI is a non-factor per
above):

1. **AS50340 Selectel-MSK** (Cloud Platform) — Arelion + RETN +
   RASCOM v6, AMS-IX v6, MSK-IX v6, Qrator. 3-4 hop v6 path via
   AS1299.
2. **AS9123 TimeWeb** — Arelion + RETN + Cogent v6. Comparable
   to Selectel-MSK.
3. **AS49505 Selectel (default VDS)** — solid RU-domestic v6 mesh
   but no direct AS1299 v6 peer (+1 hop typically: 49505 → 9002 →
   1299 → 16509).
4. **AS25532 Mastertel** — strong PITER-IX presence (FRA, Helsinki,
   Tallinn, Riga) gives interesting non-Arelion alt-paths.
5. **AS216246 Aeza (RU)** — narrower transit (RETN-RU + GNM); v6
   path 5+ hops.
6. **AS197695 REG.RU / RuVDS** — mostly RU-domestic v6; longest
   paths to Stockholm.

### Notable surprises

- **TimeWeb's v6 upstream mix is arguably better than Selectel's
  main AS49505** (Cogent + Arelion + RETN). Selectel's "good" v6
  AS is the MSK-specific AS50340 (Cloud Platform product), not
  the default VDS line.
- **Aeza's "Tier-1 multihomed" marketing** is mostly about its
  NL/DE international entity (AS210644), not the RU-located
  AS216246 which has narrow transit.
- **Mastertel's PITER-IX-Frankfurt v6 presence** is an underrated
  feature — non-Arelion alt-path to AWS via FRA.

## Touched concept pages

- [[hosting-provider-as-dpi-variable]] — TSPU-in-ISP positioning
  means provider's local v6 transit doesn't move the DPI needle
- [[dpi-flow-learning]] — the TSPU v6 parity finding supplements
  the v4 observations
- [[reality-protocol]] — the AI-keyed REALITY detector (Dec 2025)
  is an attack surface this concept page should track

## Sources

- [bgp.he.net/AS49505 — Selectel](https://bgp.he.net/AS49505)
- [bgp.tools/as/50340 — Selectel-MSK](https://bgp.tools/as/50340)
- [bgp.tools/as/9123 — TimeWeb](https://bgp.tools/as/9123)
- [bgp.tools/as/210644 — Aeza Intl](https://bgp.tools/as/210644)
- [bgp.tools/as/216246 — Aeza Group LLC](https://bgp.tools/as/216246)
- [bgp.tools/as/197695 — REG.RU/RuVDS](https://bgp.tools/as/197695)
- [bgp.tools/as/25532 — Mastertel](https://bgp.tools/as/25532)
- [PeeringDB AS16509 AWS](https://www.peeringdb.com/net/1418)
- [RIPE Stat looking-glass `2a05:d016::/36`](https://stat.ripe.net/data/looking-glass/data.json?resource=2a05:d016::/36)
- [RIPE Stat looking-glass `2a05:d014::/36`](https://stat.ripe.net/data/looking-glass/data.json?resource=2a05:d014::/36)
- [RIPE Stat looking-glass `13.51.0.0/16` (v4 eu-north-1)](https://stat.ripe.net/data/looking-glass/data.json?resource=13.51.0.0/16)
- [TSPU DPI Analysis — deepwiki/rcd27](https://deepwiki.com/rcd27/zapret2-mcp/3.5-tspu-deep-packet-inspection-analysis)
- [Netnod IX Stockholm](https://www.netnod.se/ix/stockholm)
- [AWS ip-ranges.json](https://ip-ranges.amazonaws.com/ip-ranges.json)
