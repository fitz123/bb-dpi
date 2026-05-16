---
tags: [community, ru, tspu, xray, vless, reality, chain-relay, 2025, 2026]
sources: []
updated: 2026-05-16
---

# Source: 2025-2026 community reports on xray RU chain relays

Survey of community research (forums, blogs, GitHub) compiled May
2026 against the question: which RU hosting providers do
practitioners recommend for xray/VLESS+REALITY chain relays, what
to avoid, and what's the state of RU DPI events affecting
provider choice.

- **Type**: external research, multi-source corroboration weighted
  over single-source claims
- **Subject**: practitioner consensus, dated DPI events, IPv6 as a
  bypass primitive
- **Ingested**: 2026-05-16
- **Companion sources**: [[s-2026-05-ru-vps-ipv6-procurement-scan]],
  [[s-2026-05-ipv6-bgp-path-aws-stockholm]]

## Provider consensus (2026)

### Multi-source positive

- **Selectel** — appears in every 2026 chain-relay reference's
  generic-RU-VPS pool ([Sergei-thinker/vpn-setup](https://github.com/Sergei-thinker/vpn-setup) Apr 2026,
  [vpsindex blog](https://vpsindex.ru/blog/xray-vless-reality-vps)).
  Safest specific recommendation for SPB/Moscow VLESS+REALITY in
  2026. Existing operational fleet success on this provider
  corroborates.
- **TimeWeb / VDSina** — listed in the same chain-relay pools.
  Caveat: same source also flags both in the "комплаянс с RKN —
  могут заблокировать VPN" list. Both are tactically viable, both
  carry RKN-compliance risk.

### Stale advice (was recommended in 2025, reversed by 2026)

- **Yandex Cloud** — Feb 2025 Habr guide explicitly endorsed
  ([Habr 990206](https://habr.com/en/articles/990206/)). Reversed
  Apr 2026 ([Sergei-thinker README](https://github.com/Sergei-thinker/vpn-setup)):
  - `AS Yandex.Cloud LLC` is filtered by TSPU separately from
    `AS YANDEX LLC` (i.e., sub-AS granularity in TSPU's filter
    tables)
  - YC VM IPs are pre-blocked in regions running active mobile
    whitelists
  Treat as a 2025 endorsement that does not survive into 2026.

### Hard avoid

- **Aeza** — Dec 2025 mass termination wave (multi-source: Gazeta,
  LowEndTalk, Habr, Cyberhub). RKN issued an IP list in
  `138.124/16` and 24-hour takedown notices. Aeza complied. RKN
  reportedly detects xray on "clean" networks even where no
  WireGuard/OpenVPN ever ran there.
- **VK Cloud** — IP ranges fail whitelist-bypass tests as of Mar
  2026 ([russia-whitelist disc#21](https://github.com/kort0881/russia-whitelist/discussions/21)).
- **Aeza, VDSina, REG.RU, TimeWeb** all on Sergei-thinker's "may
  block VPN on RKN request" list. Tactically viable for relay role
  but bounded by takedown-compliance.

## The chain-relay pattern in 2026

The RU-box → foreign-exit cascade is well-documented in 2026
community guides:

- [Habr 990206 (Feb 2025)](https://habr.com/en/articles/990206/) —
  canonical Russian-language guide
- [ntc.party 23943 — VLESS Reality каскадом через ру VPS](https://ntc.party/t/vless-reality-каскадом-через-ру-vps/23943)
- [Sergei-thinker/vpn-setup](https://github.com/Sergei-thinker/vpn-setup)
  (v1.10.1 as of May 2026) — full multi-layer chain automation
- [nozikov/vless-relay-setup](https://github.com/nozikov/vless-relay-setup) —
  generic 2-VPS chain automation
- [net4people/bbs #490](https://github.com/net4people/bbs/issues/490) —
  explains the TSPU dynamic that makes the chain necessary

Convergence: **TSPU is more lenient on RU↔RU server-to-server
traffic than RU→foreign-DC**. The 15-20KB foreign-IP freeze rule
(below) does not apply to RU-anchored TCP. This is the structural
reason for the chain-relay design.

## IPv6 as a DPI bypass — weak public evidence

- Only one source mentions IPv6 in this context:
  [Habr 990206](https://habr.com/en/articles/990206/) suggests
  "add IPv6 to the exit node" specifically as a workaround for
  app-compat with locked-down apps (WhatsApp, Instagram). No
  measurement, no claim that v6 itself defeats DPI.
- No 2026 chain-relay automation guide treats IPv6 as a
  load-bearing knob.
- **Conclusion**: no public measurement-based evidence that
  switching a relay's chain-leg to v6 measurably improves DPI
  evasion. Combined with [[s-2026-05-ipv6-bgp-path-aws-stockholm]]'s
  finding of TSPU v6 inspection parity (Mar 2026), the v6-as-bypass
  hypothesis is unsupported by public evidence in 2026.

## Camouflage SNI guidance (2026 community)

- **Generic `google.com` SNI is failing on Aeza-hosted REALITY**
  ([Habr QnA 1404636](https://qna.habr.com/q/1404636), Oct 2025) —
  matches the operator's own [[asn-match-sni-camouflage]] doctrine.
- **`vkvideo.ru` recommended for RU-anchored REALITY** (Habr 990206) —
  plausible same-AS match.
- [meower1/Reality-SNI-Finder](https://github.com/meower1/Reality-SNI-finder) —
  community tool to discover working SNIs per ASN.
- No public source publishes a per-provider SNI matrix; practitioners
  test against the local ASN with the finder tool.

## Dated DPI events affecting provider choice

A timeline of measurement-grounded RU DPI changes in the 12 months
ending May 2026:

| Date | Event | Source |
|---|---|---|
| Jun 2025 | TSPU 15-20KB foreign-IP freeze: TLS-1.3 to non-whitelist IPs (Hetzner, DO, AWS, OVH) silently choked above ~15-20KB. Drove the RU-relay design. | [net4people/bbs #490](https://github.com/net4people/bbs/issues/490) |
| Jul 2025 | Aeza OFAC-sanctioned. | [BleepingComputer](https://www.bleepingcomputer.com/news/security/aeza-group-sanctioned-for-hosting-ransomware-infostealer-servers/) |
| Oct 2025 | RKN reports 258 VPN services blocked YTD. | [www1.ru](https://www1.ru/en/news/2025/10/26/rkn-soobshhil-o-blokirovke-258-vpn-servisov-za-tekushhii-god.html) |
| Dec 2025 | Aeza mass-terminates VPN customers on RKN list. | [LowEndTalk 212584](https://lowendtalk.com/discussion/212584/aeza-has-started-blocking-users-for-vpn-usage) |
| Dec 2025 | TSPU actively blocking VLESS, SOCKS5, L2TP. | [Cyberhub](https://www.cyberhub.blog/article/16470-russias-roskomnadzor-actively-blocking-vless-socks5-and-l2tp-vpn-protocols), [Mezha](https://mezha.net/eng/bukvy/russia-begins-blocking-vless-vpn-protocol-increasing-internet-restrictions/) |
| Dec 2025 | RKN deploys AI-keyed VLESS+REALITY detector on TLS-1.3 to non-whitelisted foreign IPs (per [[s-2026-05-ipv6-bgp-path-aws-stockholm]]). | deepwiki/rcd27 |
| Jan 2026 | RKN block list reaches 439 VPN services, +70% in 3 months. | [www1.ru](https://www1.ru/en/news/2026/01/22/roskomnadzor-ogranicil-dostup-k-439-vpn-servisam-v-rossii.html) |
| Mar 2026 | TSPU IPv6 inspection parity reached. | deepwiki/rcd27 |
| Mar 2026 | VK Cloud whitelist workarounds stop working. | [russia-whitelist disc#21](https://github.com/kort0881/russia-whitelist/discussions/21) |
| Apr 2026 | Mobile whitelist enforcement: Gosuslugi, Sberbank, Yandex services deny VPN clients on mobile. | [Meduza](https://meduza.io/en/feature/2026/04/30/russia-blocks-vpn-access-to-major-platforms-moves-to-charge-for-mobile-vpn-traffic), [Zona.media](https://en.zona.media/article/2026/04/07/russian_internet_censorship_2026) |

## xray-core countermeasure (worth pinning)

- **xray-core 25.12.8+** adds `testpre` and `testseed` defenses
  intended to defeat the timing + entropy ML detector deployed by
  RKN (Dec 2025). Pin minimum version on both ends of the chain.

## Touched concept pages

- [[dpi-flow-learning]] — adds 15-20KB foreign-IP freeze (Jun 2025)
  and AI-keyed detector (Dec 2025) as new mechanisms
- [[reality-protocol]] — AI-keyed REALITY detector is a new attack
  surface
- [[xhttp-transport]] — chain-relay pattern works because TSPU
  treats RU↔RU server-to-server traffic differently from
  RU→foreign-DC; XHTTP framing on the foreign leg is part of the
  defense
- [[asn-match-sni-camouflage]] — Yandex Cloud sub-AS filtering
  (TSPU distinguishes `AS Yandex.Cloud LLC` from `AS YANDEX LLC`)
  is a refinement of the ASN-correlation attack model
- [[hosting-provider-as-dpi-variable]] — multi-source corroboration
  for the concept

## Sources

- [Habr 990206 — chain setup guide, Feb 2025](https://habr.com/en/articles/990206/)
- [Sergei-thinker/vpn-setup, May 2026](https://github.com/Sergei-thinker/vpn-setup)
- [nozikov/vless-relay-setup](https://github.com/nozikov/vless-relay-setup)
- [net4people/bbs #490 — TSPU foreign-IP freeze](https://github.com/net4people/bbs/issues/490)
- [LowEndTalk — Aeza terminations](https://lowendtalk.com/discussion/212584/aeza-has-started-blocking-users-for-vpn-usage)
- [Gazeta.ru — Aeza/RKN, Dec 2025](https://www.gazeta.ru/tech/news/2025/12/07/27354103.shtml)
- [Habr QnA 1404636 — Aeza+REALITY SNI](https://qna.habr.com/q/1404636)
- [ntc.party 23943 — каскад через RU VPS](https://ntc.party/t/vless-reality-каскадом-через-ру-vps/23943)
- [ntc.party 24230 — VPS surviving whitelists](https://ntc.party/t/vpsvds-работающие-при-белых-списках/24230)
- [meower1/Reality-SNI-Finder](https://github.com/meower1/Reality-SNI-finder)
- [Meduza — mobile VPN block, Apr 2026](https://meduza.io/en/feature/2026/04/30/russia-blocks-vpn-access-to-major-platforms-moves-to-charge-for-mobile-vpn-traffic)
- [Cyberhub — VLESS/SOCKS5/L2TP blocking](https://www.cyberhub.blog/article/16470-russias-roskomnadzor-actively-blocking-vless-socks5-and-l2tp-vpn-protocols)
- [www1.ru — 439 VPN services blocked Jan 2026](https://www1.ru/en/news/2026/01/22/roskomnadzor-ogranicil-dostup-k-439-vpn-servisam-v-rossii.html)
- [www1.ru — 258 VPN services blocked Oct 2025](https://www1.ru/en/news/2025/10/26/rkn-soobshhil-o-blokirovke-258-vpn-servisov-za-tekushhii-god.html)
- [Habr 990236 — DPI analysis](https://habr.com/en/articles/990236/)
- [VPSindex — VLESS+xray+Reality on VPS](https://vpsindex.ru/blog/xray-vless-reality-vps)
- [CodeHummus — Aeza vs Timeweb Cloud](https://www.codehummus.com/blog/03-best-vps-for-vpn/)
- [Zona.media — RU internet censorship 2026](https://en.zona.media/article/2026/04/07/russian_internet_censorship_2026)
- [TSPU DPI Analysis — deepwiki/rcd27](https://deepwiki.com/rcd27/zapret2-mcp/3.5-tspu-deep-packet-inspection-analysis)
- [russia-whitelist disc#21 — VK Cloud workarounds stop](https://github.com/kort0881/russia-whitelist/discussions/21)
