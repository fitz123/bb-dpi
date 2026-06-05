---
tags: [hosting-provider, procurement, ru, multi-reviewer, methodology]
sources: [s-2026-05-ru-vps-ipv6-procurement-scan, s-2026-05-tspu-asn-camouflage-research, s-2026-05-xray-relay-community-reports]
updated: 2026-05-22
---

# Source: 2026-05 multi-reviewer alt-research on RU VDS shortlist

Adversarial multi-reviewer pass over a single-researcher candidate
shortlist for an additional RU-domestic chain-relay node. Three
independent reviewers (Codex via `codex exec`, an Opus subagent via
`Task`, and Gemini via `gemini --approval-mode plan`) were given the
lead's shortlist plus the same prompt asking for missed alternatives,
critiques of the picks, extra anti-candidates, and methodological
weaknesses. JSON output, parallel dispatch (~3 min wall-clock).

- **Type**: multi-reviewer adversarial review
- **Subject**: candidate RU VDS providers for a second RU-domestic
  relay, given an existing relay on Selectel AS49505
- **Ingested**: 2026-05-22
- **Companion sources**: [[s-2026-05-ru-vps-ipv6-procurement-scan]] (the
  May-2026 single-researcher market scan that this exercise
  audited a sibling-shortlist of), [[s-2026-05-xray-relay-community-reports]],
  [[s-2026-05-tspu-asn-camouflage-research]]

## Method

Lead researcher (a separate Claude-Opus agent earlier in the same
session) produced a 5-provider shortlist with ASNs, prices, payment
methods, fate-isolation ranking, and a "match cover-site brand to
provider AS tenant archetype" cover-site-design heuristic.

Three reviewers received the lead's full output and the same prompt:
critique it; propose alternatives; flag anti-candidates; assess the
cover-site heuristic. JSON schema required `verdict ∈ {concur,
dissent, partial}` plus structured `alternatives_proposed`,
`critiques_of_lead`, `extra_anti_candidates`,
`cover_site_narrative_take`, `final_recommendation`.

PII discipline: no fleet-specific identifiers in the prompt. The
existing relay is referenced only as "the AS49505 relay". No active
probes; reviewers worked from public RDAP/BGP/PeeringDB lookups and
vendor pricing pages.

## Verdicts

- **Codex**: `partial` — RUVDS still defensible as primary *conditional
  on RDAP confirmation at allocation*, Beget runner-up call is weak,
  FirstByte should be dropped.
- **Opus**: `dissent` — RUVDS #1 ranking is invalid because the lead's
  own report internally lists AS197695 as REG.RU's ASN (an
  excluded provider), and RUVDS is a brand on top of that
  infrastructure.
- **Gemini**: `dissent` — RUVDS does verified phone-KYC and suspends
  VPN-shaped tenants; Beget shares PITER-IX with Selectel; Timeweb
  does aggressive auto-suspends. Proposes JustHost in Novosibirsk for
  maximum geo-isolation.

## Convergent findings

### 1. AS-to-provider attribution is not stable (multiple reviewers)

The lead listed RUVDS as AS197695, but also listed AS197695 as REG.RU
in its own exclusions section — an internal contradiction the lead
self-acknowledged but did not resolve. Opus surfaced this directly;
Codex left RUVDS at AS197695 but flagged "verify allocated IP" at
checkout. The [[s-2026-05-ru-vps-ipv6-procurement-scan]] sibling
research used different ASNs again (VDSina AS216071 instead of
AS48282; FirstByte AS50979 instead of AS204997).

**Implication**: a single whois lookup or vendor marketing page is
not authoritative for AS-to-provider mapping in the RU VDS market —
many providers operate on multiple ASes, lease prefixes from peers,
or share infrastructure with related brands. RDAP-of-the-allocated-IP
at provisioning time is the only ground truth. See
[[hosting-provider-as-dpi-variable#procurement-verification]].

### 2. Crypto-payment claims are routinely overstated (Codex-verified)

Codex pulled official vendor payment pages and found:

- **Beget**: card / SBP / YuMoney / Robokassa only — no crypto. Lead
  had claimed "BTC/USDT".
- **Timeweb Cloud**: card / SBP / YuMoney / SberPay / invoices — no
  crypto. Lead had claimed "crypto via aggregator".
- **VDSina**: crypto via the Heleket aggregator with
  community-reported ~29–38% effective fees (Opus, LowEndTalk).
- **FirstByte**: PayPal-only per firstbyte.pro/info/payments — and
  PayPal has been unavailable in Russia since March 2022 for new
  accounts.

**Implication**: "crypto-friendly" claims on RU-VDS comparison
pages and LLM-generated shortlists routinely overstate native
support. Either confirm against the vendor's own
checkout/payments page or treat payment as RUB-card-only and budget
the friction.

### 3. FirstByte pattern-matches the recently-sanctioned bulletproof-host structure (Opus)

UK shell company over RU operations is the exact pattern that the
November 2025 US/UK/AU coordinated sanctions targeted (Media Land,
following the July 2025 Aeza sanctions per
[[s-2026-05-tspu-asn-camouflage-research]]). Combined with the
broken payment surface, FirstByte was demoted from the lead's #5 to
the anti-candidate list by both Opus and Codex.

### 4. SPb-IX overlap is a real fate-sharing axis (all three reviewers)

Beget peers at PITER-IX in Saint Petersburg; Selectel does too. For a
second relay whose purpose is to survive a Selectel-level failure,
the IXP/transit overlap is non-trivial. Codex and Opus both
downgraded Beget for this; Gemini independently proposed JustHost in
Novosibirsk explicitly for the geographic/IX orthogonality.

This is a refinement to axis 1 of
[[hosting-provider-as-dpi-variable]]: same-AS SNI pairing is one
dimension of provider choice; physical/IXP infrastructure overlap is
another, and they are not equivalent.

### 5. The "AS tenant archetype" cover-site heuristic is overstated (Codex + Opus)

The lead argued the cover-site brand should match the provider's
typical-tenant archetype (RUVDS = fintech tenants → use a fintech
cover; Beget = WordPress studios → use a portfolio cover). Codex and
Opus both pushed back: provider tenant mixes are wide enough that
"generic small-business / docs / portfolio / CMS landing page" is
plausible on essentially any of the candidate ASes. The
load-bearing cover-site discipline is *don't clone the existing
cover-site identically* (camouflage diversity), not *match cover to
AS sociology*.

### 6. Physical-DC overlap at L1 is a separate fate-sharing axis (Opus)

DataPro Moscow houses VDSina, HOSTKEY, and several other RU hosts at
the building/power/upstream-fiber level. Two providers on different
ASes inside the same DC are NOT fully fate-isolated. Choose at most
one tenant per major RU DC for any fate-isolation argument to hold.

## Alternatives proposed (cross-reviewer)

Sorted by how many reviewers independently suggested each:

| Provider | ASN(s) per reviewers | Cities | Proposed by | Notes |
|---|---|---|---|---|
| AdminVPS | AS211183 / AS59729 (varied) | Moscow | All 3 | Mainstream tenant blend; crypto story ambiguous; needs checkout verification |
| DataCheap (Delta) | AS16262 | Moscow + Kazan + Novosibirsk | Codex + Opus | Own Tier-III DC + own ASN + long history; SBP-only payment (no native crypto per Codex) |
| HOSTKEY | AS57043 | Moscow (DataPro) | Codex + Opus | Foreign legal vehicle = jurisdictional orthogonality; BitPay crypto; **DC overlap with VDSina** |
| JustHost (Baxet) | AS51659 (Gemini) / AS207651 (Opus) | Novosibirsk / Kazan / Khabarovsk | Gemini + Opus | Far-east PoPs = maximum geo-isolation; ASN attribution disagreement between reviewers — needs RDAP |
| FirstVDS (JSC IOT) | AS29182 / AS62200 | Irkutsk | Codex | Out of SPb/Moscow IX cluster; payment privacy weak |
| ProfitServer | AS47655 | Moscow + multi-region | Opus | Mskhost-tier mainstream; crypto-friendly |
| SmartApe | AS56694 | Moscow | Gemini | Shared-hosting noise for blend |

## Anti-candidates (cross-reviewer)

Beyond the lead's existing excludes (Aeza, PQ.hosting, 4vps.su,
Reg.ru, Selectel), the reviewers added:

- **FirstByte (AS204997)** — Opus + Codex: broken payment surface +
  sanctioned-pattern legal structure.
- **Inferno Solutions (AS200487)** — proposed by Gemini, rejected by
  Opus: Stockholm court seized Inferno servers for TA505 hosting.
  Same risk class as Aeza, lower public profile.
- **Yandex Cloud / VK Cloud / SberCloud / MTS Cloud** — Codex: high
  KYC, fast RKN compliance, structurally hostile to minimal-KYC
  relay use.
- **RuWeb** — Codex: ToS explicitly bans anonymous-proxy / shell /
  Tor / similar services.
- **DDoS-Guard-heavy reseller paths** — Codex: bad camouflage
  neighborhood for circumvention traffic; upstream reputation
  dominates ASN choice.
- **No-KYC reseller storefronts (generic)** — Codex: ASN /
  jurisdiction unverifiable without RDAP-of-allocated-IP at
  provisioning time; often resell or geo-spoof inventory.

## Re-ranked recommendation

The lead's RUVDS-primary recommendation does not survive review.
Synthesised ranking after the multi-reviewer pass:

1. **DataCheap (AS16262, Moscow)** — clean own-DC + own-ASN + Kazan /
   Novosibirsk geographic options. SBP-only payment is the cost.
2. **AdminVPS (AS211183, Moscow)** — three-reviewer convergence;
   crypto path ambiguous, needs checkout verification.
3. **HOSTKEY (AS57043, Moscow)** — only if BitPay/KYC friction is
   acceptable; *do not also use VDSina* due to DataPro L1 overlap.
4. **JustHost far-east PoP** (Khabarovsk / Novosibirsk) — maximal
   geo-isolation; ASN attribution disagreement between reviewers,
   needs RDAP verification at allocation.
5. **RUVDS** — conditional only: verify the allocated IP is not on
   AS197695 (REG.RU) at checkout time. If RUVDS owns a separate AS
   for its KVM line, that AS is the candidate; if it's reselling
   REG.RU space, fate-isolation is invalidated.

## Touched concept pages

- [[hosting-provider-as-dpi-variable]] — adds a procurement-verification
  axis (RDAP-of-allocated-IP, payment-rail-as-friction, DC overlap at
  L1) on top of the three existing variables.
- [[asn-match-sni-camouflage]] — the AS-attribution ambiguity result
  reinforces "verify the actual allocated-IP ASN, not vendor marketing"
  for SNI-pool selection.

## Sources

- [Aeza VPN terminations (LowEndTalk)](https://lowendtalk.com/discussion/212584/aeza-has-started-blocking-users-for-vpn-usage)
- [US Treasury OFAC on Aeza, Jul 2025](https://home.treasury.gov/news/press-releases/sb0185)
- [Media Land sanctioned Nov 2025 (TechCrunch)](https://techcrunch.com/2025/11/19/us-uk-and-australia-sanction-russian-bulletproof-web-host-used-in-ransomware-attacks/)
- [Inferno Solutions Stockholm seizure (Website Planet)](https://www.websiteplanet.com/web-hosting/inferno-solutions/)
- [Beget official payment FAQ](https://beget.com/ru/kb/faq/general/obshhie-voprosy-raznoe)
- [Timeweb Cloud VDS pricing](https://timeweb.cloud/vds-vps)
- [FirstByte payment page](https://firstbyte.pro/info/payments/)
- [Cloudflare Radar AS204997 (FirstByte)](https://radar.cloudflare.com/as204997/)
- [DataCheap payment methods](https://datacheap.ru/about/payments/)
- [HOSTKEY Bitcoin VPS](https://hostkey.com/vps/bitcoin-vps/)
- [bgp.he.net AS57043 (HOSTKEY)](https://bgp.he.net/AS57043)
- [FirstVDS official payments](https://firstvds.ru/company/payments)
- [AdminVPS crypto page](https://adminvps.ru/vps/vps_crypto.php)
- [PayPal suspended in Russia, Mar 2022 (CNBC)](https://www.cnbc.com/2022/03/05/paypal-suspends-its-services-in-russia-over-ukraine-war.html)
- [PeeringDB AS198610 (Beget)](https://www.peeringdb.com/asn/198610)
