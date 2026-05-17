---
tags: [dns, ipv6, tspu, macos, libc, observability]
sources: [s-tool-rkn-block-checker, s-2026-05-tspu-asn-camouflage-research]
updated: 2026-05-17
---

# DNS AAAA cascade failure (macOS libc)

A specific failure mode where macOS apps using libc's `getaddrinfo`
(curl, Python, Go runtime, most browsers' default resolver path)
silently fail to resolve a hostname **even though the DNS data is fully
available and `dig` returns a valid answer**. The cascade is:
`getaddrinfo(host, family=AF_INET)` on macOS still issues a parallel
AAAA query (Apple's happy-eyeballs reality, not a strict A-only); if
that AAAA query times out on any of the configured resolvers, the
entire lookup is treated as failed and the calling app receives
`gaierror(8, 'nodename nor servname provided, or not known')`.

This is **exploitable by an adversary** that can selectively drop or
hang AAAA queries on a public-DNS path that macOS happens to pick.

## The mechanism

macOS's `libsystem_info` resolver does not honour the `AF_INET` family
hint as a strict A-only query. When an app calls:

```python
socket.getaddrinfo("host.example", None, family=socket.AF_INET)
```

the resolver internally issues both A and AAAA queries against the
configured resolver chain (`scutil --dns` resolver #1) to determine
preference. If AAAA times out on the resolver macOS happens to send
it to, the AAAA timeout cascades:

- the A answer arrives but is not returned in isolation
- `getaddrinfo` returns `gaierror`
- the calling app sees an apparent DNS failure

A raw DNS client like `dig host.example` sends an A-only query
explicitly, never issues AAAA, and returns the answer in milliseconds.
This produces the diagnostic asymmetry: **`dig` works, apps don't**.

## How TSPU exploits it (observed 2026-05-17)

A first-party reproduction on a RU-consumer-network vantage demonstrated:

1. `dig +short www.vtb.ru @1.1.1.1` → returns `195.242.83.13` cleanly.
2. `dig +short @8.8.8.8 AAAA www.vtb.ru` → **probabilistic timeout**
   (one query in N hangs; the others return clean NOERROR-no-answer).
3. `python3 socket.getaddrinfo("www.vtb.ru", None, family=AF_INET)` →
   `gaierror` when the AAAA-on-8.8.8.8 leg hangs.
4. `curl https://www.vtb.ru/` from the same shell → `Could not resolve
   host`.

The pattern is consistent with TSPU's documented banking/government
heightened-scrutiny list and the broader IPv4/IPv6 differential
treatment ([[s-2026-05-tspu-asn-camouflage-research]],
[[s-2026-05-ipv6-bgp-path-aws-stockholm]]). The DPI drops AAAA queries
to public-DNS endpoints for specific high-scrutiny hostnames; the
cascade exploits a macOS resolver quirk to promote that into an
A-record-equivalent failure.

The probability is **not 100%** — many AAAA queries get clean responses.
The failure surface is "1-in-N apps fail to resolve" not "every app
fails every time", which makes it operationally insidious: users see
intermittent "site won't load" for banking and government sites that
work for everyone else, and the symptom is hard to attribute because
the next reload often works.

## Why the wiki cares

This is the documented mechanism behind the
[[s-tool-rkn-block-checker]] verdict `✗ DNS — system DNS doesn't
resolve, DoH does — consistent with DNS poisoning`. The tool's
heuristic correctly classifies the failure as DNS-layer (system path
fails, Cloudflare DoH succeeds), but the underlying mechanism **isn't
classical DNS poisoning** (returning a wrong IP); it's an adversarial
denial-of-service on AAAA queries that exploits a macOS-specific
resolver quirk.

The wiki preserves the distinction because:

- "DNS poisoning" suggests "fix the upstream resolver" → wrong
  mitigation here (the upstream resolver IS returning correct data on
  A queries; the issue is AAAA-side selective drop combined with libc).
- The actual fix targets the libc cascade (skip AAAA, change
  resolvers, or tunnel DNS via DoH).

## Observable signatures

Diagnostic checklist for "site won't open in browser but works in dig":

| Signal | Meaning |
|---|---|
| `dig +short host` returns IP | DNS data is reachable; not classical poisoning |
| `dig +short @8.8.8.8 AAAA host` hangs intermittently | TSPU AAAA-drop on this hostname / resolver path |
| `python -c "import socket; socket.getaddrinfo(host, None, family=socket.AF_INET)"` raises gaierror | libc cascade fires |
| `curl host` fails "Could not resolve" | App-level confirmation |
| `dscacheutil -q host -a name host` is empty | mDNSResponder doesn't have a positive cache (consistent with cascade, not stale negative) |
| Apex (`example.ru`) works but `www.example.ru` doesn't | The DPI AAAA-drop list operates on full FQDNs; the apex may not be on the list |

## Mitigations

In order of ROI:

1. **Keep a DNS-intercepting VPN tunnel active.** A sing-box / xray
   TUN that captures DNS at the TUN layer and forwards to a DoH
   resolver completely bypasses the libc-vs-8.8.8.8 chain — the libc
   resolver never reaches a TSPU-affected upstream because sing-box
   serves the answer locally from its own DNS subsystem. The
   [[s-memory-chain-relay-rationale]] split-DNS pattern (`.ru` →
   russia-DoH) does this. When active, AAAA-cascade isn't reachable
   as an attack surface.
2. **Drop the AAAA-affected public resolver from system DNS.** On
   macOS: System Settings → Wi-Fi → Details → DNS, remove `8.8.8.8`,
   keep `1.1.1.1` and `8.8.4.4`. Reduces exposure but `8.8.4.4` could
   also become drop-affected, and DHCP-pushed resolvers may overwrite
   the static config on every join.
3. **Pin macOS to a single DoH resolver via a configuration profile.**
   Apple's encrypted-DNS mechanism (`NEDNSSettings` in a
   `.mobileconfig`) supports per-system DoH/DoT. A profile pointing
   at `dns.comss.one` or another RU-friendly DoH eliminates the
   public-DNS path entirely for VPN-off state.
4. **Force-disable AAAA in the libc chain.** macOS has no clean
   `disable-ipv6` toggle for libc; some apps respect `RES_OPTIONS` or
   custom builds with `c-ares` accept `family=AF_INET` strictly, but
   coverage is incomplete. Not recommended as a primary mitigation.

## Limits & open questions

- The AAAA-drop probability is empirically <100% and not measured
  tightly. A formal longitudinal study via cron'd
  [[s-tool-rkn-block-checker]] would help size mitigation urgency.
- Unknown whether TSPU AAAA-drop targets:
  - Specific FQDNs from a blocklist (most likely, given the
    banking/gov-host pattern)
  - All `.ru` AAAA queries to public DNS (possible)
  - Random sampling (less likely given the consistent host-specific
    pattern in this observation)
- macOS version-specific. The AAAA-cascade-on-`AF_INET` behavior has
  been verified on Python 3.14 on macOS Sonoma/Sequoia era (late
  2025 / early 2026). Older macOS may behave differently; Apple has
  historically tightened/loosened this in different releases.
- Linux glibc and musl behaviour unverified here. Linux's
  `getaddrinfo` with `AF_INET` is more often strict A-only; the
  cascade may be macOS-specific.
- iOS behaviour unverified.

## Sources

- [[s-tool-rkn-block-checker]] — the tool whose verdict surfaced the
  symptom and motivated this investigation; its `dns.py` uses
  `socket.getaddrinfo(host, None, family=AF_INET)` (the affected
  path) for the system-resolver leg.
- First-party reproduction on 2026-05-17, RU consumer-network vantage.
- [[s-2026-05-tspu-asn-camouflage-research]] — TSPU's
  banking/government-host scrutiny tier (the destination side of the
  attack).
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]] — IPv4/IPv6 differential
  treatment landscape (the v6-targeting side; the README's
  "v6 less filtered" claim is the *other* face of the same
  v6-targeting behavior).
- Apple `getaddrinfo(3)` man page (system-level documentation of the
  resolver chain).
