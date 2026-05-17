---
tags: [dns, ipv6, macos, libc, observability, hypothesis, invalidated]
sources: [s-tool-rkn-block-checker, s-2026-05-tspu-asn-camouflage-research]
updated: 2026-05-17
---

# DNS AAAA cascade failure — invalidated hypothesis

This page documents an **invalidated mechanism hypothesis** for a
transient gaierror symptom observed once on 2026-05-17. The wiki
preserves it because the methodology mistake is generalisable: a
single observation got interpreted under a wrong baseline assumption,
the resulting hypothesis was codified, and re-verification a few hours
later invalidated both the persistence of the symptom and the
mechanism. Keep this as a teaching case for how *not* to write
concept pages from one observation.

## What was observed (the fact)

On a RU-consumer-network vantage Mac running sing-box with TUN
auto-route active, an `rkn-check` run returned `✗ DNS — system DNS
doesn't resolve, DoH does` for `www.vtb.ru`. Verified at the time by:

- `dig +short www.vtb.ru @1.1.1.1` → returned `195.242.83.13`
  cleanly (raw A-query bypassing libc resolver).
- `dig +short @8.8.8.8 AAAA www.vtb.ru` → timed out on the first
  attempt; subsequent retries returned clean NOERROR-no-answer.
- `python3 socket.getaddrinfo("www.vtb.ru", None, family=AF_INET)`
  → `gaierror(8, 'nodename nor servname provided, or not known')`.
- `curl https://www.vtb.ru/` → "Could not resolve host".
- `dscacheutil -flushcache` and retry: still failed.

## Disconfirming re-verification (2026-05-17, hours later)

A second run from the same vantage minutes-to-hours later:

- Five RU banking/government hosts (`www.vtb.ru`, `www.sberbank.ru`,
  `www.gov.ru`, `yandex.ru`, `www.alfabank.ru`) queried for
  `dig @8.8.8.8 AAAA <host>` — **all five returned clean** (no
  timeouts).
- Even more decisively, on a follow-up validation pass:
  `getaddrinfo("www.vtb.ru", AF_INET)` succeeded 5/5 times in a row,
  the `rkn-check` verdict for `www.vtb.ru` flipped to `✓ OK`, and
  sing-box log showed clean `dns: exchanged A www.vtb.ru. 4448 IN A
  195.242.83.13`.

**The symptom did not reproduce.** The original gaierror was a
transient.

## Why the original mechanism hypothesis is invalidated

The initial concept page proposed: "macOS libc `getaddrinfo(AF_INET)`
issues parallel AAAA; TSPU drops AAAA queries to 8.8.8.8 selectively
for banking hostnames; the AAAA timeout cascades into a full
getaddrinfo failure." That story rested on an unstated assumption:
*the libc resolver was talking directly to 8.8.8.8*.

That assumption was **wrong**. sing-box was running with TUN
auto-route enabled. macOS's system resolver was nominally configured
with `1.1.1.1 / 8.8.8.8 / 8.8.4.4` (per `scutil --dns`), but
`route -n get 1.1.1.1` and `route -n get 8.8.8.8` both pointed at
`utun7` (the sing-box TUN interface). Every libc DNS query was
intercepted by sing-box before leaving the box and re-routed via
sing-box's own DNS rules — for `.ru` hosts, that means the
`russia-dns` server (DoH to `dns.comss.one`), **not** raw UDP to
8.8.8.8.

So the failure could not have been "TSPU drops AAAA at 8.8.8.8"
because no packet was ever sent to 8.8.8.8. The proposed mechanism
contradicts the routing facts.

## What this leaves us with

The factual observation is real but unattributed. Candidate mechanisms
that *do* fit the corrected baseline:

| Hypothesis | Why it could fit | How to distinguish |
|---|---|---|
| **sing-box russia-dns DoH transient** | sing-box was trying to reach `dns.comss.one` for `www.vtb.ru` AAAA; the DoH call (or its bootstrap A lookup via `direct-dns`) hung or returned SERVFAIL once; sing-box returned the error to libc; libc surfaced `gaierror`. | Enable sing-box `level: debug` log, reproduce, look for upstream-selection + timeout/SERVFAIL entries. |
| **dns.comss.one upstream selective AAAA failure** | The Russian DoH provider may have its own quirks for AAAA on banking hostnames; one-time SERVFAIL or 4xx response. | Probe `https://dns.comss.one/dns-query` directly for `www.vtb.ru AAAA` with multiple attempts. |
| **mDNSResponder libc-level negative caching** | The libc resolver path may cache "host unresolvable" for some period even after sing-box returns the correct A. `dscacheutil -flushcache` doesn't reach all caches on modern macOS. | `sudo killall mDNSResponder` (full restart), then retry the failing host immediately. |
| **Transient packet loss on the utun7 path** | One-time network blip between libc and sing-box-internal DNS server (172.19.0.1) caused a timeout. | Hard to retro-attribute; symptom is gone. |

None of these is currently confirmed. Each predicts a different
recovery pattern, and the page deliberately does not pick a winner.

## Lessons (the actual reason this page is kept)

1. **Always document baseline routing before interpreting symptoms.**
   The original observation conflated "system DNS configured at
   1.1.1.1" with "system DNS reaching 1.1.1.1". Auto-route makes
   those two different. A 30-second `route -n get <resolver>` check
   would have rescued the hypothesis from the start.

2. **Don't codify a single observation as a named mechanism.** One
   gaierror on one host at one moment is not a mechanism. The
   Karpathy-pattern wiki preserves uncertainty by design — single
   observations belong in a log entry or a synthesis-page "open
   question", not in a `concepts/` page with a stable slug other
   pages link to.

3. **Re-verification is cheap and catches false positives.** The
   re-verification that invalidated this hypothesis took 30 seconds
   (`for i in 1 2 3 4 5; do python3 -c ...; done`). It would have
   prevented the entire write-up if run earlier.

## Operational guidance (independent of the invalidated mechanism)

If you see a "dig works, apps don't" symptom again:

1. **First**: run `route -n get 8.8.8.8` (and 1.1.1.1) on the affected
   Mac. If they point at `utun*`, sing-box is intercepting — the
   query never reaches the listed resolver. Investigate sing-box's
   DNS path, not the public resolver.
2. **Second**: query sing-box's clash-api at
   `http://127.0.0.1:9090/dns/query?name=<host>&type=A` and `type=AAAA`
   to see what sing-box's internal resolver returns. If it returns
   cleanly, the failure is between sing-box and libc (mDNSResponder
   cache, macOS resolver shim quirks).
3. **Third**: `sudo killall mDNSResponder` for a full cache restart
   (more reliable than `dscacheutil -flushcache`).
4. **Fourth**: only after the above, consider raising sing-box log
   level to `debug` (requires config edit + restart, which costs a
   reconnect window).

## Status

- **Hypothesis (AAAA-cascade-on-public-DNS-AAAA-drop):** invalidated
  by routing facts.
- **Symptom (transient gaierror on www.vtb.ru):** real but
  unreproduced; no confirmed mechanism.
- **Page status:** preserved as a teaching case, not as an
  authoritative concept. Other pages should not link to this as a
  citable mechanism.

## Sources

- [[s-tool-rkn-block-checker]] — the tool whose `✗ DNS` verdict
  surfaced the original symptom.
- First-party investigation and re-verification, 2026-05-17, RU
  consumer vantage with sing-box TUN auto-route active.
- [[s-2026-05-tspu-asn-camouflage-research]] — the broader RU DPI
  context the original (now invalidated) hypothesis tried to
  connect to.
