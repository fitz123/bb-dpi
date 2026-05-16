---
tags: [hosting-provider, asn, procurement, dpi-evasion, ru]
sources: [s-2026-05-ru-vps-ipv6-procurement-scan, s-2026-05-tspu-asn-camouflage-research, s-2026-05-xray-relay-community-reports, s-2026-05-ipv6-bgp-path-aws-stockholm]
updated: 2026-05-16
---

# Hosting provider as a DPI-evasion variable

A chain relay's hosting provider is not just a procurement decision —
it is a DPI-evasion variable that materially shapes the relay's
fingerprint, its longevity, and the camouflage primitives available
to it. Treat it with the same rigour as protocol or SNI choice.

## The three axes

### 1. Source-ASN ↔ camouflage-SNI pairing

Per [[asn-match-sni-camouflage]], TSPU performs an SNI/IP-correspondence
check: it compares the destination IP's ASN against the SNI's
authoritative DNS-answer ASN, and flags mismatches. The relay's
*hosting-provider ASN* therefore constrains which SNIs can plausibly
be claimed without tripping that check.

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

## Empirical signal

The operator's "generic Google-CDN SNI on a cloud-region exit" drop
incident ([[s-memory-sni-asn-correlation-incident]]) is consistent
with the SNI/IP-correspondence mechanism documented in
[[s-2026-05-tspu-asn-camouflage-research]]: cloud-provider IP plus a
non-matching CDN SNI is a maximally-mismatched
(destination_ASN, SNI_authoritative_ASN) tuple. Fixing the SNI to one
whose DNS answer resolves into the cloud provider's regional pool
reversed the drop, exactly as the mechanism would predict.

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
- IPv6 is *not* a fourth axis here: TSPU reached IPv6 inspection
  parity Mar 2026 ([[s-2026-05-ipv6-bgp-path-aws-stockholm]]),
  so dual-stack is opportunistic latency optimisation, not DPI
  evasion.

## Sources

- [[s-2026-05-ru-vps-ipv6-procurement-scan]]
- [[s-2026-05-tspu-asn-camouflage-research]]
- [[s-2026-05-xray-relay-community-reports]]
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]]
- [[s-memory-sni-asn-correlation-incident]]
- [[s-memory-chain-relay-rationale]]
