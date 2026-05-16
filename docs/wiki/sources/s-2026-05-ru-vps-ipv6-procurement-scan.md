---
tags: [hosting-provider, procurement, ipv6, ru]
sources: []
updated: 2026-05-16
---

# Source: May 2026 RU VPS market scan for native dual-stack v4+v6

External research compiled May 2026 against the question: which RU-located
VPS providers ship native dual-stack IPv4+IPv6 *out of the box* (no
support-ticket dance, no paid v6 addon) suitable for an xray REALITY
chain-relay role.

- **Type**: external web research, multi-source per provider
- **Subject**: provider procurement for a v6-enabled RU chain-relay
- **Ingested**: 2026-05-16
- **Companion sources**: [[s-2026-05-ipv6-bgp-path-aws-stockholm]],
  [[s-2026-05-xray-relay-community-reports]]

## Method

Multi-source check per provider: official pricing/spec page,
PeeringDB ASN record, LowEndTalk / Habr / ntc.party 2025-2026 threads,
RKN/OFAC enforcement actions. Targets: ₽250-700/mo, KVM, Ubuntu
24.04, root SSH, no 443/8443 blocks, dual-stack default.

## Provider landscape, distilled

### Default dual-stack (no ticket required)

| Provider | ASN | DC | Cheapest ₽/mo | v6 footprint |
|---|---|---|---|---|
| VDSina | AS216071 | Moscow (Datapro) | ~₽280 | Free /64, own ASN announces |
| TimeWeb Cloud | AS9123/AS47845 | MSK, SPB, +12 DCs | ₽450 | Free v6 in MSK/SPB tiers, default |
| FirstByte | AS50979 | Moscow | ₽75-219 | 1× v4 + 1× v6 included; +1₽ for extra v6 up to 256 |
| Beget | AS198610 | SPB, MSK, KZ, LV | ₽210 | /64 default; AS announces only 1 v6 prefix (limited footprint) |
| Hostkey | AS57043 | NL/US/RU(Moscow) | ~₽270 (€3) | /64 free on KVM tier |

### v6 only on ticket / paid addon (disqualifies for "default-on" criterion)

- **Selectel VDS-2 / VDS line** (the operator's existing relay
  product) — included IP is "v4 OR v6", not both. Additional IP
  addon ~₽178/mo per Selectel marketing. Migration to Selectel
  Cloud Platform (AS50340) gets ticket-based v6 enable per their
  docs ([Selectel KB setup-ipv-6](https://kb.selectel.ru/docs/cloud/servers/networks/setup-ipv-6/)).
- **FirstVDS** — v6 on request, +₽20/mo for v6 subnet.
- **IHC.host** — /64 = +₽100/mo.
- **ProfitServer** — v6 is an "option to add", not default.
- **Reg.ru cloud** — v6 availability per plan, not clearly default.

### Hard exclusions

- **Aeza** — disqualified twice:
  - OFAC-sanctioned **July 2025** as a bulletproof host
    ([US Treasury press release](https://home.treasury.gov/news/press-releases/sb0185),
    [BleepingComputer](https://www.bleepingcomputer.com/news/security/aeza-group-sanctioned-for-hosting-ransomware-infostealer-servers/))
  - Mass-terminated VPN customers on RKN list **December 2025**
    ([LowEndTalk 212584](https://lowendtalk.com/discussion/212584/aeza-has-started-blocking-users-for-vpn-usage),
    [Habr news](https://habr.com/ru/news/973644/),
    [Xakep](https://xakep.ru/2025/12/05/aeza-vpn/),
    [Gazeta.ru](https://www.gazeta.ru/tech/news/2025/12/07/27354103.shtml))
- **Yandex Cloud** — previously recommended (Feb 2025 Habr guide) but
  reversed Apr 2026: TSPU now filters `AS Yandex.Cloud LLC` separately
  from `AS YANDEX LLC`, and YC IPs are pre-blocked in regions running
  active whitelists. See [[s-2026-05-xray-relay-community-reports]].
- **VK Cloud** — IPs fail whitelist-bypass tests as of Mar 2026
  ([github.com/kort0881/russia-whitelist disc#21](https://github.com/kort0881/russia-whitelist/discussions/21)).
- **3v-Hosting, Stellarbyte, Eternalhost, Mchost, Ihor** —
  insufficient 2025-2026 signal to qualify as production-grade.
- **Cloud4Y** — wrong product class (enterprise IaaS).

## Top picks for the role

1. **VDSina (AS216071)** — own ASN with native v6 prefix announce,
   Moscow Datapro DC, KVM, ₽ billing. Different ASN from AS49505/AS50340
   = a different BGP path for chain-leg testing.
2. **TimeWeb Cloud (AS9123)** — explicit "v6 free in MSK/SPB", 14 DCs
   give site choice. Caveat: documented to remove resources on RKN
   request.
3. **FirstByte (AS50979)** — cheapest with true dual-stack-by-default,
   lower-profile ASN = potentially less RKN attention.

Selectel's *Cloud Platform* (AS50340, not the VDS line on AS49505)
is the most operationally-similar option to the existing relay —
same provider, same billing — at the cost of a one-time support
ticket to enable v6. Worth considering if the goal is "minimum change
to the deploy flow".

## Caveats

- **Hypothesis-invalidating finding from companion source**: TSPU
  reportedly achieved IPv6 inspection parity in March 2026
  ([[s-2026-05-ipv6-bgp-path-aws-stockholm#section-6]]). Public
  community research found no measurement-based evidence that
  switching a chain-relay's outbound to v6 measurably improves DPI
  evasion ([[s-2026-05-xray-relay-community-reports#section-4]]).
  Provider choice still matters for path latency/stability, but
  v6-as-bypass should NOT be the primary procurement rationale.
- "Default-on v6" status was checked at marketing-page level; verify
  at order time before committing — pages may reflect a different
  tier or region than the cheapest plan.
- Port 443/8443 blocking is *not documented* for any candidate.
  RKN enforcement in 2025-2026 is takedown-notice based, not L4
  port-block based.
- Provider stability bound is set by RKN-compliance behavior. See
  [[hosting-provider-as-dpi-variable]] for the generalisable
  framework.

## Touched concept pages

- [[hosting-provider-as-dpi-variable]] — defines the variable this
  research operationalises
- [[asn-match-sni-camouflage]] — provider choice constrains the SNI
  pool available for camouflage on the new relay's ASN

## Sources

- [Aeza VPN terminations (LowEndTalk)](https://lowendtalk.com/discussion/212584/aeza-has-started-blocking-users-for-vpn-usage)
- [Aeza VPN terminations (Habr)](https://habr.com/ru/news/973644/)
- [US Treasury OFAC on Aeza, Jul 2025](https://home.treasury.gov/news/press-releases/sb0185)
- [BleepingComputer: Aeza sanctioned](https://www.bleepingcomputer.com/news/security/aeza-group-sanctioned-for-hosting-ransomware-infostealer-servers/)
- [TimeWeb Cloud official prices](https://timeweb.cloud/prices)
- [Habr 2025 RU VPS cheap-tier comparison](https://habr.com/en/articles/990310/)
- [RuVDS official Linux from ₽130](https://ruvds.com/en-usd/linux)
- [FirstByte KVM SSD VPS](https://firstbyte.ru/vps-vds/kvm-ssd/)
- [FirstVDS official tariffs](https://firstvds.ru/products/vds_vps_hosting)
- [VDSina official pricing](https://vdsina.ru/en/pricing)
- [VDSina ASN 216071 PeeringDB](https://www.peeringdb.com/asn/216071)
- [Beget VPS official](https://beget.com/en/vps)
- [Beget ASN 198610 PeeringDB](https://www.peeringdb.com/asn/198610)
- [Hostkey IPv6 VPS](https://hostkey.com/vps/ipv6/)
- [ProfitServer Russia VPS](https://profitserver.net/vps/russia/)
- [Selectel KB — setup IPv6](https://kb.selectel.ru/docs/cloud/servers/networks/setup-ipv-6/)
- [Selectel cheap VPS marketing](https://selectel.ru/services/cloud/servers/cheap/)
