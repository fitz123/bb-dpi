---
tags: [hosting-provider, asn, procurement, dpi-evasion, ru]
sources: [s-2026-05-ru-vps-ipv6-procurement-scan, s-2026-05-tspu-asn-camouflage-research, s-2026-05-xray-relay-community-reports, s-2026-05-ipv6-bgp-path-aws-stockholm, s-2026-05-multi-reviewer-vds-shortlist]
updated: 2026-05-22
---

# Hosting provider as a DPI-evasion variable

A chain relay's hosting provider is not just a procurement decision —
it is a DPI-evasion variable that materially shapes the relay's
fingerprint, its longevity, and the camouflage primitives available
to it. Treat it with the same rigour as protocol or SNI choice.

## The three axes

### 1. Source-ASN ↔ camouflage-SNI pairing

Per [[asn-match-sni-camouflage]], TSPU is community-reported to
perform an SNI/IP-correspondence check: comparing the destination
IP's ASN against the SNI's authoritative DNS-answer ASN, and
flagging mismatches. The mechanism is a community hypothesis; the
observable failure-mode (mismatched SNI/IP gets dropped, matched
SNI/IP gets through) is first-party measured. Either way, the
relay's *hosting-provider ASN* constrains which SNIs can plausibly
be claimed without tripping the heuristic.

The strongest pairing is when the SNI's CDN pool resolves into the
relay's own provider ASN — e.g., a Yandex-property SNI on a Yandex
Cloud relay, or a Selectel-hosted hostname on a Selectel relay.
This is the highest-leverage source-side move *per documented
mechanism*.

The provider choice gates this entire camouflage primitive: a relay
on a provider with no plausible same-AS public SNI is structurally
constrained to weaker camouflage.

### 2. Compliance posture (relay longevity)

RU hosting providers fall on a spectrum of how they respond to RKN
takedown demands for VPN-running customers:

- **Active purge** — Aeza, Dec 2025: mass-terminated VPN customers
  on the `138.124/16` range under RKN list, demanded screenshots
  of deletion ([[s-2026-05-tspu-asn-camouflage-research]]). Worst
  longevity bet.
- **Pre-emptive cooperation** — RuVDS CEO has publicly stated RKN
  "recommendations may become requirements"; telegraphs more
  cooperative compliance.
- **Reactive compliance** — Selectel, TimeWeb, FirstVDS, Beget:
  no public 2025-2026 mass-takedown event documented. They comply
  on specific RKN orders, but no proactive sweeps.
- **Low-profile / niche** — small providers (FirstByte, VDSina) get
  fewer RKN-targeted requests by volume but offer no structural
  protection if specifically named.

This axis bounds relay lifetime. A relay on Aeza was never going
to last; a relay on a reactive-compliance provider may last months
or years.

### 3. TSPU inspection profile of the source ASN

Even with a perfect same-AS SNI match, some provider ASNs sit
inside TSPU's destination-side filter set or trigger pre-emptive
mobile-whitelist blocks:

- **Selectel (AS49505 / AS50340)** — documented mid-2025 side-channel:
  TSPU's filtering "in its zeal began to cut off regular HTTPS
  traffic to Russian hosting providers, affecting access to web
  servers on Selectel within Russia"
  ([[s-2026-05-tspu-asn-camouflage-research]]). Inside the
  inspection set, but not pre-emptively blocked.
- **Yandex Cloud (AS Yandex.Cloud LLC, separate from AS YANDEX LLC)** —
  TSPU filters the sub-AS separately, and YC VM IPs are
  pre-blocked in regions running active mobile whitelists
  ([[s-2026-05-xray-relay-community-reports]]). The strong
  SNI/IP-pairing benefit (axis 1) is partly negated by the
  active-inspection cost.
- **VK Cloud** — whitelist-bypass workarounds stopped working
  Mar 2026; IPs fail the test.
- **General-purpose hosts (TimeWeb, RuVDS, Beget, FirstByte,
  VDSina)** — no specific TSPU-elevation documented.

This axis is in tension with axis 1: the providers with the best
SNI-pairing potential (Yandex Cloud for yandex-*, VK Cloud for
vk.*) are also the most-inspected.

## Procurement verification (axis-independent prerequisite)

The three axes above presume the *provider* claimed by the vendor's
marketing matches the *AS* the allocated IP actually announces from,
and that headline payment claims match the vendor's actual checkout
flow. Neither is reliable in the RU VDS market.

Empirically observed in 2026-05 multi-reviewer pass
([[s-2026-05-multi-reviewer-vds-shortlist]]):

- **AS-to-provider attribution is unstable**. A vendor brand may
  operate on multiple ASes, lease prefixes from peers, or sit on top
  of a related-but-distinct AS-holding's infrastructure. Three
  independent reviewers and a parallel single-researcher market scan
  ([[s-2026-05-ru-vps-ipv6-procurement-scan]]) disagreed on the
  authoritative ASN for at least three of the same provider names
  (VDSina, FirstByte, JustHost). Vendor marketing pages are not
  ground truth.
- **Headline "crypto-friendly" claims routinely overstate**. In a
  five-provider sample, three providers had no native crypto per
  their own checkout pages despite being widely referenced as
  crypto-accepting on comparison sites; a fourth had aggregator-gated
  crypto with ~30%+ effective fees per community reports.
- **DC overlap at L1 is a separate fate-sharing axis from the AS**.
  DataPro Moscow houses multiple providers on different ASes; two
  candidates inside the same DC building/power/upstream-fiber are
  not L1-fate-isolated even if their ASNs are.

The pre-purchase checklist that addresses these:

1. **RDAP of the allocated test-IP** (not vendor marketing). Verify
   the announced ASN against the candidate ASN-disjoint set.
2. **Official vendor payment page**, not third-party shortlists.
3. **Vendor ToS scan** for explicit anti-anonymous-proxy clauses
   (some providers, e.g. RuWeb, explicitly ban; others permit by
   silence).
4. **DC building cross-reference** against datacentermap.com — avoid
   stacking two candidates inside the same physical facility.
5. **Two-source ASN agreement** (bgp.tools / bgp.he.net / PeeringDB /
   ipip.net) before treating the brand-to-AS mapping as load-
   bearing. If the sources disagree, fate-isolation must be
   evaluated against the *underlying* AS, not the brand.
6. **Upstream-transit chain inspection.** A candidate can have its
   own announced AS *and* transit via an excluded provider's AS.
   Pull the upstreams list from bgp.he.net's AS page and verify
   none of them are on the exclusion set. Discovered empirically
   2026-05-22 when AdminVPS (AS211183, own announced AS) was found
   transiting via REG.RU (AS197695); the multi-review's "two-source
   ASN agreement" check would not catch this because the announce-
   from AS was correct — the fate-sharing lived at the upstream
   layer. See [[2026-05-ru-vds-shortlist-multi-review#validation-pass-2026-05-22]].

This is not a fourth DPI variable — it is meta-discipline that
applies to all three axes above. Skipping it can silently invalidate
a provider choice that looked correct on paper.

## Cover-site narrative — what's load-bearing and what isn't

A separate axis at the cover-site layer ([[reality-protocol]] `dest`
camouflage): does the cover-site brand need to *match* the
provider's typical-tenant archetype? E.g., should a relay on a
fintech-VPS-heavy provider only carry a fintech-themed cover?

Empirically: **no, this heuristic is overstated**
([[s-2026-05-multi-reviewer-vds-shortlist]]). Provider tenant mixes
are wide enough that a generic small-business / documentation /
portfolio / CMS landing page is plausible on essentially any of the
candidate ASes. The genuinely load-bearing cover-site discipline is:

- **Camouflage diversity** — don't run the *same* cover-site on
  multiple relays. If both relays land on the same brand, an
  adversary that learns one has learned both.
- **Coherence with the certificate** — SNI, CN/SAN, served body, and
  HTTP headers must agree. This is what active-probe-resistant
  REALITY camouflage actually depends on.

Matching brand to "AS sociology" is at most a tie-breaker. Not
worth optimising for in early procurement.

## Empirical signal

The operator's "generic Google-CDN SNI on a cloud-region exit" drop
incident ([[s-memory-sni-asn-correlation-incident]]) is consistent
with the SNI/IP-correspondence mechanism community-described in
[[s-2026-05-tspu-asn-camouflage-research]]: cloud-provider IP plus a
non-matching CDN SNI is a maximally-mismatched
(destination_ASN, SNI_authoritative_ASN) tuple. Fixing the SNI to one
whose DNS answer resolves into the cloud provider's regional pool
reversed the drop, exactly as the hypothesised mechanism would
predict — first-party measured outcome, community-theorised cause.

## How to apply the variable

1. Identify candidate provider ASNs (see procurement scan at
   [[s-2026-05-ru-vps-ipv6-procurement-scan]]).
2. For each candidate, enumerate plausible same-AS public SNIs
   with CDN pools that resolve into the provider's IP space.
3. Cross-check against axis 3: is the provider's AS on TSPU's
   active-inspection list?
4. Weight by axis 2: provider's compliance posture caps the
   relay's expected lifetime — match procurement effort to that.
5. **A/B if uncertain.** The Yandex Cloud trade-off in particular
   is best resolved by running two relays simultaneously (one
   YC + yandex-* SNI, one Selectel + selectel-AS SNI) and
   measuring which survives.

## Limits

- This concept describes a *generalisable variable*, not a
  one-time procurement checklist. The picture shifts with each
  TSPU rollout and each RKN enforcement wave.
- Pending RU draft legislation (Apr 2026) would shift all RU
  hosting providers from "technical intermediary" to "controller"
  status — uniform hostile compliance duty. If passed, axis 2
  collapses to "all RU providers comply quickly", and the variable
  becomes weaker.
- The variable interacts with — does not replace — protocol-layer
  ([[reality-protocol]]), SNI-layer ([[asn-match-sni-camouflage]]),
  and transport-layer ([[xhttp-transport]]) camouflages. Don't
  optimise provider choice in isolation.
- IPv6 is *not* a fourth axis here: TSPU is reported to have reached
  IPv6 inspection parity Mar 2026
  ([[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]] —
  single-source claim, pending corroboration), so dual-stack is
  opportunistic latency optimisation, not DPI evasion. Even on the
  upside (claim doesn't hold), the companion community-reports
  source separately found no measurement-based evidence that v6
  helps — the procurement implication is the same.

## Sources

- [[s-2026-05-ru-vps-ipv6-procurement-scan]]
- [[s-2026-05-tspu-asn-camouflage-research]]
- [[s-2026-05-xray-relay-community-reports]]
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]]
- [[s-memory-sni-asn-correlation-incident]]
- [[s-memory-chain-relay-rationale]]
