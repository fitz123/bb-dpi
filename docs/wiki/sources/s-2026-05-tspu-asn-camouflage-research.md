---
tags: [tspu, asn, sni-correspondence, fingerprinting, ipv6, ru, provider-compliance]
sources: [s-2026-05-ru-vps-ipv6-procurement-scan, s-2026-05-ipv6-bgp-path-aws-stockholm, s-2026-05-xray-relay-community-reports]
updated: 2026-05-16
---

# Source: 2025-2026 TSPU fingerprinting + provider-ASN research

Measurement-grounded survey compiled May 2026 of public research on
RU TSPU fingerprinting techniques and per-provider inspection
posture, with weight given to OONI / academic / first-party
disclosures over operator-claim guides.

- **Type**: external research, measurement-weighted
- **Subject**: TSPU heuristics, provider-ASN inspection profile,
  v6 differential treatment, provider compliance posture
- **Ingested**: 2026-05-16
- **Companion sources**: [[s-2026-05-ru-vps-ipv6-procurement-scan]],
  [[s-2026-05-ipv6-bgp-path-aws-stockholm]],
  [[s-2026-05-xray-relay-community-reports]]

## TSPU fingerprinting techniques (measurement-grounded)

### Destination-side heuristics (dominant)

- **TLS-1.3 + foreign-DC + 15-20 KB freeze** (late 2025): a TLS-1.3
  connection to a non-whitelisted foreign DC freezes silently once
  it passes ~15-20 KB. No RST; server-side packets stop arriving.
  Foreign DCs named explicitly: Hetzner, DO, OVH, Cloudflare;
  AWS observed independently by this project.
  ([net4people #490](https://github.com/net4people/bbs/issues/490),
  [Habr 990236](https://habr.com/en/articles/990236/))
- **CIDR whitelist on destination** (added late 2025): TSPU is
  applying a destination-CIDR allowlist as a coarse pre-filter
  ([net4people #490](https://github.com/net4people/bbs/issues/490)).
  Direct support for [[asn-match-sni-camouflage]]'s
  destination-ASN-correlation model.
- **SNI/IP correspondence check** (*community hypothesis,
  consistent with first-party observation*): per a Habr Q&A
  community discussion ([Habr QnA 1404636](https://qna.habr.com/q/1404636)),
  TSPU is described as comparing destination IP with the SNI's real
  DNS answer and dropping on mismatch. This is *the named
  mechanism* community discussion uses to explain the operator's
  first-party "generic-CDN-SNI on a cloud-region exit" drop
  ([[s-memory-sni-asn-correlation-incident]]) — the observed effect
  is real and measured; the named mechanism is the most plausible
  community theory but is not from OONI, academic measurement, or a
  reproducible test. Evidence grade: forum-sourced explanation of a
  first-party-observed failure. The downstream
  [[asn-match-sni-camouflage]] guidance is consistent with both the
  observed effect AND the named mechanism, so it's the right
  operational response either way.

### Per-flow heuristics

- **Empty SNI lifts block 100%** ([Habr 990236](https://habr.com/en/articles/990236/)) —
  confirms TSPU's filter is *SNI-presence + heuristic*, not pure
  blocklist. Empty-SNI flows are not interesting enough to inspect.
- **Port 443 vs high ports**: identical VLESS+REALITY config on 443
  → instant drop / zero throughput; same config on 47000+ → ~80%
  pass-through ([Habr 990236](https://habr.com/en/articles/990236/)).
  TSPU's 443 sampling rate is much higher.
- **Active probing of REALITY**: REALITY forwards probes to the
  cover host by design. No 2025-2026 *measurement* paper publishes
  a successful REALITY active-probe attack by TSPU. Theoretical
  signals (subtle ClientHello extension-order artifacts) remain
  speculative ([XTLS discussion #3269](https://github.com/XTLS/Xray-core/discussions/3269)).

### Architectural facts

- **TSPU is deployed in-ISP, not in transit**. The middlebox sees
  the packet before it leaves the RU AS. Transit-ASN diversity
  doesn't help.
- **Heterogeneity**: hardware/software versions co-exist across
  ISPs → regional inconsistency. What works on home-ISP-X in
  Moscow may fail on mobile-Beeline in Izhevsk.
  ([HRW 2025 report](https://www.hrw.org/report/2025/07/30/disrupted-throttled-and-blocked/state-censorship-control-and-increasing-isolation),
  [TSPU IMC22 paper](https://ensa.fi/papers/tspu-imc22.pdf))
- **ECH blocked** Nov 2024 ([OONI](https://ooni.org/post/2024-russia-report/)).
- **Mobile whitelist mode**: full-CIDR allowlist on mobile,
  ICMP filtered, custom-VPS SNI blocked even with valid TLS;
  only Yandex/VK/CDNVideo/Beeline ranges traverse
  ([net4people #516](https://github.com/net4people/bbs/issues/516)).

## Provider-ASN inspection profile

### Measurement-grounded

- **Selectel (AS49505 / AS50340)**: documented as a side-channel
  victim of mid-2025 TSPU rollout — "TSPU in its zeal began to cut
  off regular HTTPS traffic to Russian hosting providers,
  affecting access to web servers on Selectel within Russia"
  ([dev.to mirror of Habr 990236](https://dev.to/shinomontaz/russias-internet-filtering-infrastructure-evolution-and-architecture-cel)).
  Implies Selectel ASNs are *inside* the TSPU inspection set, not
  whitelisted. No active customer purge documented.
- **Aeza (AS210644)**: bulletproof reputation. OFAC sanctioned
  2025-07-01 ([Treasury sb0185](https://home.treasury.gov/news/press-releases/sb0185)).
  Post-sanctions ASN shift to AS211522 Hypercore Ltd detected
  2025-07-20 ([Silent Push IoFA](https://www.silentpush.com/news/iofa-detects-aeza-group-infrastructure/)).
  Then Dec 2025 self-purge of VPN customers on `138.124/16`
  under RKN pressure.
- **TimeWeb (AS9123)**: long-standing abuse-list presence (RIPE
  anti-abuse-wg complaint thread 2020, [CleanTalk AS9123](https://cleantalk.org/blacklists/as9123)).
  Not bulletproof, but not whitelisted either.
- **RuVDS**: CEO has publicly said RKN "recommendations may
  become requirements" — telegraphs more-cooperative compliance
  posture ([Kommersant 8364064](https://www.kommersant.ru/doc/8364064)).
- **Hostkey**: markets "VPS for VPN" openly ([hostkey.com/vps/vpn](https://hostkey.com/vps/vpn/)).
  Soft signal of tolerance — also of being on RKN's radar.

### Operator-claim only (lower weight)

- **Yandex Cloud (AS13238 / AS208722)** — Feb 2025 Habr guide
  ([Habr 990206](https://habr.com/en/amp/publications/990206/))
  endorsed YC as "most reliable", BUT see contradictory finding
  in [[s-2026-05-xray-relay-community-reports]]: TSPU now
  filters `AS Yandex.Cloud LLC` separately from `AS YANDEX LLC`
  (sub-AS granularity), and YC VM IPs are pre-blocked in regions
  running mobile whitelists. Trade-off: YC gives the strongest
  *same-ASN-as-SNI* match (yandex.* SNIs resolve in YC's AS pool)
  but the AS itself is on the inspection radar.
- **VK Cloud** — operator-claim "free traffic, hard to get
  whitelisted IPs"; March 2026 community reports the
  whitelist-bypass workarounds stopped working.

## v6 vs v4 differential treatment

- **No measurement-grade study** exists in the public corpus. Both
  TSPU IMC22 paper and net4people threads are v4-centric.
- **Operator-claim signal**: RU mobile operators "haven't invested
  in IPv6 infrastructure, whitelisting on it hasn't been tested"
  — i.e., v6 not blocked because rarely deployed/measured, not
  because TSPU is lenient
  ([igareck README](https://github.com/igareck/vpn-configs-for-russia/blob/main/README-EN-US.md)).
- This source's v6 read: opportunistic dual-stack, not a strategy.
  Consistent with the single-sourced TSPU v6 parity claim
  summarised in
  [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]]
  (Mar 2026, pending corroboration).

## Provider compliance posture

| Provider | Behavior | Evidence |
|---|---|---|
| Aeza | Comply within 24h; demand screenshots of deletion | [Habr 973644](https://habr.com/ru/news/973644/), [Xakep](https://xakep.ru/2025/12/05/aeza-vpn/) |
| RuVDS | Pre-emptively cooperative (CEO statements) | [Kommersant 8364064](https://www.kommersant.ru/doc/8364064) |
| Selectel, TimeWeb, FirstVDS, Beget | No public 2025-2026 mass-takedown event; comply on specific RKN orders | — |

System-wide: 258 VPN services blocked in 2025 (+31% YoY); 439 by
Jan 2026 (+70% in 3 months).

**Pending legislation (Apr 2026)**: Kommersant via Meduza reports
draft law would shift RU hosting providers from "technical
intermediary" to "controller" status — required to proactively
prevent VPN-capacity supply
([Meduza](https://meduza.io/en/news/2026/04/17/kommersant-reports-russia-seeks-to-ban-hosting-providers-from-supplying-computing-capacity-to-vpn-operators)).
If passed, all RU providers face structurally hostile compliance
duty.

## Camouflage SNI hygiene (2026)

- General hygiene: `google.com` burnt; `yandex.ru` "often detected
  now"; current rotation: `microsoft.com`, `vk.com`, `apple.com`,
  `vkvideo.ru`, `tbank.ru`, `kinopoisk.ru`, `spotify.com`. SNI
  lifespan: single-day to multi-week.
- **Highest-leverage move per the community-hypothesised
  mechanism**: the best camouflage SNI is one whose *real DNS
  answer* falls in the *same ASN* as the relay's IP. This targets
  the hypothesised SNI/IP correspondence check directly (and is
  consistent with the first-party measured drop / fix pattern in
  [[s-memory-sni-asn-correlation-incident]]). Selectel-hosted
  relay + Selectel-hosted-SNI ≠ Yandex+yandex.ru; for Yandex SNI
  you need a Yandex-AS relay.

## Contradictions / open questions

- **Yandex Cloud — primary action lever or avoid-list?**
  - For: SNI/IP correspondence puts YC + yandex-* SNI in a
    privileged position; the documented heuristic punishes
    AS-mismatch directly (this source).
  - Against: YC sub-AS is on TSPU's separate filter (see
    [[s-2026-05-xray-relay-community-reports]]); YC IPs
    pre-blocked in mobile-whitelist regions (Apr 2026).
  - Unresolved without first-party testing. The operator can
    A/B a YC relay against a Selectel relay with respective
    same-AS SNIs.
- **VLESS protocol-level blocking** — "VLESS blocked"
  (zona.media) vs "VLESS still works with right config" (NTC
  threads). Resolution: blocking is heuristic + regional;
  outright protocol ban is not in evidence; the term
  "VLESS-blocked" in RU press conflates protocol-fingerprinting
  attacks with the broader inspection set.
- **No measurement source isolates source-ASN scrutiny** as a
  primary TSPU axis. Destination-side heuristics + SNI/IP
  correspondence dominate the evidence weight.

## Operator-actionable distillation

1. **The destination-side heuristics are the load-bearing
   constraint**, not the source ASN. Tuning the source ASN is
   a secondary lever.
2. **The highest-leverage source-side move** is matching the
   relay's ASN to the camouflage SNI's authoritative ASN. This
   targets the hypothesised SNI/IP correspondence check and is
   consistent with the first-party measured drop / fix pattern in
   [[s-memory-sni-asn-correlation-incident]].
3. **Avoid Aeza/Hypercore** (active customer purge + bulletproof
   stigma).
4. **Treat v6 as opportunistic, not a bypass strategy**.
5. **Avoid foreign-cloud destinations on the next hop** (AWS,
   Hetzner, DO, OVH) — they're inside TSPU's destination
   blocklist plus 15-20 KB freeze rule.
6. **The pending Apr 2026 hosting-as-controller legislation**
   would change the compliance landscape uniformly — bake an
   exit plan into the relay design.

## Touched concept pages

- [[asn-match-sni-camouflage]] — adds the SNI/IP-correspondence
  mechanism details and the source-ASN-matches-SNI-ASN refinement
- [[dpi-flow-learning]] — adds 15-20 KB freeze, port-bias
  heuristic, empty-SNI exemption
- [[reality-protocol]] — port-443 selection-bias and the
  AI-keyed REALITY detector (Dec 2025) are new attack surfaces
- [[hosting-provider-as-dpi-variable]] — primary corroboration
  source for the concept

## Sources

- [Habr 990236 — DPI analysis](https://habr.com/en/articles/990236/)
- [dev.to mirror of 990236](https://dev.to/shinomontaz/russias-internet-filtering-infrastructure-evolution-and-architecture-cel)
- [Habr 990206 — chain setup guide](https://habr.com/en/amp/publications/990206/)
- [Habr QnA 1404636 — Aeza+REALITY SNI](https://qna.habr.com/q/1404636)
- [net4people #490 — TSPU foreign-IP freeze + CIDR whitelist](https://github.com/net4people/bbs/issues/490)
- [net4people #516 — mobile whitelist mode](https://github.com/net4people/bbs/issues/516)
- [TSPU IMC22 paper](https://ensa.fi/papers/tspu-imc22.pdf)
- [HRW 2025 — Disrupted, Throttled, and Blocked](https://www.hrw.org/report/2025/07/30/disrupted-throttled-and-blocked/state-censorship-control-and-increasing-isolation)
- [OONI RU 2024 report](https://ooni.org/post/2024-russia-report/)
- [US Treasury OFAC Aeza](https://home.treasury.gov/news/press-releases/sb0185)
- [Silent Push — Aeza → Hypercore ASN shift](https://www.silentpush.com/news/iofa-detects-aeza-group-infrastructure/)
- [TRM Labs Aeza analysis](https://www.trmlabs.com/resources/blog/treasury-sanctions-global-bulletproof-hosting-service-aeza-group-for-enabling-cybercriminal-activity)
- [Habr 973644 — Aeza VPN purge](https://habr.com/ru/news/973644/)
- [Xakep — Aeza VPN](https://xakep.ru/2025/12/05/aeza-vpn/)
- [SecurityLab Aeza](https://www.securitylab.ru/news/566895.php)
- [Kommersant 8364064 — RuVDS CEO on RKN](https://www.kommersant.ru/doc/8364064)
- [Meduza — Apr 2026 hosting-as-controller draft law](https://meduza.io/en/news/2026/04/17/kommersant-reports-russia-seeks-to-ban-hosting-providers-from-supplying-computing-capacity-to-vpn-operators)
- [kort0881/russia-whitelist-bypass — RU VPS guide](https://github.com/kort0881/russia-whitelist-bypass/blob/main/guides/guides/vps-reality-setup.md)
- [kort0881/russia-whitelist disc#21](https://github.com/kort0881/russia-whitelist/discussions/21)
- [igareck/vpn-configs-for-russia](https://github.com/igareck/vpn-configs-for-russia/blob/main/README-EN-US.md)
- [XTLS discussion #3269 — REALITY probing theory](https://github.com/XTLS/Xray-core/discussions/3269)
- [Hostkey VPS-for-VPN marketing](https://hostkey.com/vps/vpn/)
- [CleanTalk AS9123 TimeWeb abuse history](https://cleantalk.org/blacklists/as9123)
- [www1.ru — 258 VPN services blocked Oct 2025](https://www1.ru/en/news/2025/10/26/rkn-soobshhil-o-blokirovke-258-vpn-servisov-za-tekushhii-god.html)
- [www1.ru — 439 VPN services blocked Jan 2026](https://www1.ru/en/news/2026/01/22/roskomnadzor-ogranicil-dostup-k-439-vpn-servisam-v-rossii.html)
