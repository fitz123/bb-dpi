---
tags: [dns, ipv6, tspu, macos, libc, observability, hypothesis]
sources: [s-tool-rkn-block-checker, s-2026-05-tspu-asn-camouflage-research]
updated: 2026-05-17
---

# DNS AAAA cascade failure (candidate mechanism, macOS libc)

A **single-observation hypothesis** for a failure class where macOS
apps using libc's `getaddrinfo` (curl, Python, Go runtime, most
browsers' default resolver path) silently fail to resolve a hostname
while `dig` returns a valid answer from the same upstream. The
hypothesised cause: `getaddrinfo(host, family=AF_INET)` is not strict
A-only on macOS — it issues parallel AAAA queries against the
configured resolver chain, and an adversary that drops AAAA selectively
on a public-DNS path that macOS happens to pick can promote that AAAA
failure into an app-level "Could not resolve".

**Status: not yet differentiated from alternatives** (see Competing
hypotheses below). The page exists so future operators can either
confirm or rule out this mechanism quickly when the symptom recurs.

## Candidate mechanism

Apple's libc resolver (`libsystem_info` / mDNSResponder path) may not
honour the `AF_INET` family hint as a strict A-only query. *If* that
is the case, when an app calls:

```python
socket.getaddrinfo("host.example", None, family=socket.AF_INET)
```

the resolver internally issues both A and AAAA queries against the
configured resolver chain (`scutil --dns` resolver #1). If AAAA times
out, the AAAA timeout *could* cascade — the A answer arrives but is
not returned in isolation, and `getaddrinfo` returns `gaierror`.

A raw DNS client like `dig host.example` sends an A-only query
explicitly, never issues AAAA, and returns the answer in milliseconds.
This produces the diagnostic asymmetry observed in the incident:
`dig` works, apps don't.

**This mechanism is not yet established** — Apple's `getaddrinfo(3)`
man page documents `PF_UNSPEC` accepting any family but does not
state that an `AF_INET` hint internally issues AAAA. Empirical
validation requires packet capture or mDNSResponder log evidence.

## Observation (2026-05-17, RU consumer-network vantage)

**Test conditions:** VPN TUN off, system DNS = `1.1.1.1`, `8.8.8.8`,
`8.8.4.4` (from DHCP / user config).

1. `dig +short www.vtb.ru @1.1.1.1` → `195.242.83.13` cleanly.
2. `dig +short @8.8.8.8 AAAA www.vtb.ru` → **timed out on the first
   try; subsequent retries returned clean NOERROR-no-answer.**
3. `python3 socket.getaddrinfo("www.vtb.ru", None, family=AF_INET)`
   → `gaierror(8, 'nodename nor servname provided, or not known')`.
4. `curl https://www.vtb.ru/` → "Could not resolve host: www.vtb.ru".
5. `dscacheutil -q host -a name www.vtb.ru` → empty.
6. `dscacheutil -flushcache` then retry getaddrinfo → still fails.

## Disconfirming evidence (follow-up sweep)

A follow-up test on five RU banking/government hosts
(`www.vtb.ru`, `www.sberbank.ru`, `www.gov.ru`, `yandex.ru`,
`www.alfabank.ru`) on the same vantage minutes later showed
**all five returned clean** from `dig +short @8.8.8.8 AAAA <host>` —
no timeouts. Yet `getaddrinfo("www.vtb.ru", family=AF_INET)` continued
to fail on that same shell.

That is **inconsistent with AAAA-timeout being the active mechanism
during the follow-up symptom**: no timeout observed, but failure
persists. This means at least one of:

- The AAAA-cascade mechanism is wrong / not the active cause.
- The AAAA-cascade fires on cached state, and the cache is stickier
  than `dscacheutil -flushcache` reaches.
- Multiple mechanisms are at play (AAAA-cascade triggered once,
  something else now keeps the host stuck).

Confidence in the AAAA-cascade mechanism is **single-observation,
unreproduced, with a counter-observation in the followup sweep**.

## Competing hypotheses

Diagnostic candidates that produce the same "dig works, apps don't"
asymmetry but via different mechanisms:

| Hypothesis | Why it could fit | How to distinguish |
|---|---|---|
| **mDNSResponder negative cache stickier than `dscacheutil -flushcache`** | The flush API may not reach all negative-cache strata; macOS has multiple cache layers (libsystem_info + mDNSResponder + AsyncDNS). | `sudo killall -INFO mDNSResponder` then `tail /var/log/system.log` to dump cache state; `sudo killall mDNSResponder` (full restart) and retry. |
| **Scoped resolver / search-domain interference** | `scutil --dns` resolver #1 may have a `search domain` set; resolvers scoped to corp domains may NXDOMAIN authoritatively before the catch-all is consulted. | `scutil --dns` and read every resolver block; compare against `dig`'s use of explicit `@<server>`. |
| **DNSSEC validation in libsystem_info** | Apple's resolver may validate AD bit; `dig` ignores AD by default. | `host -v www.vtb.ru` to see AD bit + validation chain. |
| **Profile-installed encrypted DNS** | A `.mobileconfig` (NEDNSSettings) may intercept libc calls only, leaving `dig` UDP-on-53 untouched. | `profiles -L` (needs sudo on modern macOS); check `~/Library/Preferences/com.apple.networkextension.plist` for active DoH profiles. |
| **AAAA-cascade (this page)** | Matches the *first* observation; does not match the followup sweep. | Packet capture during `getaddrinfo` call; check whether both A and AAAA queries leave the box and whether AAAA hangs. |

The diagnostic asymmetry alone does **not** uniquely identify
AAAA-cascade. The page exists as one candidate among several.

## Observable signatures

For the symptom class generally (not specifically AAAA-cascade):

- `dig +short host` returns IP → DNS data is reachable; not classical
  poisoning at the upstream.
- `python -c "import socket; socket.getaddrinfo(host, None, family=socket.AF_INET)"` raises `gaierror` → libc-level failure.
- `curl host` fails "Could not resolve" → app-level confirmation.
- See [[s-tool-rkn-block-checker]] for the tool's DNS-layer verdict
  classifier; it correctly flags the symptom as DNS-layer but its
  text "consistent with DNS poisoning" is one possible explanation
  among several, not a mechanism call.

## Mitigations

Independent of which mechanism is active, these reduce exposure:

- **DNS-intercepting VPN tunnel** (e.g., sing-box / xray TUN forwarding
  `.ru` to a DoH like `dns.comss.one`). When active, the libc resolver
  chain doesn't reach a public DNS that an adversary can manipulate.
  Note: this is the same state operators in the chain-relay setup are
  in by default; the symptom was observed VPN-off, so this mitigation
  is **plausible-by-construction** but not confirmed for AAAA-cascade
  specifically.
- **Drop the affected public resolver from system DNS** (System
  Settings → Wi-Fi → Details → DNS). If `8.8.8.8` is the bad path,
  remove it and keep `1.1.1.1` + `8.8.4.4`. DHCP-pushed resolvers
  may overwrite.
- **System DoH profile** (`NEDNSSettings` mobileconfig) pinning to a
  preferred DoH. Bypasses the libc → public-DNS path entirely.
- **Force-disable AAAA in the libc chain.** macOS has no clean
  toggle; not recommended.

## Limits & open questions

- **N=1.** A single AAAA timeout on a single host. Probability of
  recurrence, distribution across hosts, and whether TSPU is involved
  at all — all unknown.
- **No packet capture** confirming AAAA-on-AF_INET in macOS, despite
  Apple man page silence on the topic.
- **Cache-vs-cascade not differentiated** by the diagnostic steps
  taken so far.
- **Sticky failure mechanism** (the persistent gaierror after
  AAAA-timeout went away) is unexplained.
- **What would confirm:** `tcpdump` on en0 during a `getaddrinfo`
  call that fails — look for both A and AAAA queries leaving the
  box, and whether AAAA hangs while A returns. Plus
  `sudo killall mDNSResponder` to fully clear caches and retry.

## Sources

- [[s-tool-rkn-block-checker]] — the tool whose verdict surfaced the
  symptom (system getaddrinfo fails, Cloudflare DoH succeeds); its
  classifier flags DNS-layer failure correctly but the underlying
  mechanism is what this page tries to attribute.
- First-party reproduction on 2026-05-17, RU consumer-network vantage
  (VPN-off baseline).
- [[s-2026-05-tspu-asn-camouflage-research]] — TSPU's banking/
  government-host heightened-scrutiny tier (the destination side of
  any AAAA-drop hypothesis).
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]] — v4/v6 differential
  treatment landscape.
- Apple `getaddrinfo(3)` man page — silent on AF_INET internal AAAA
  behaviour; not a positive citation for the cascade hypothesis.
