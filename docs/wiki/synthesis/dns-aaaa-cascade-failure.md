---
tags: [dns, ipv6, macos, libc, methodology, hypothesis, most-likely-invalidated, teaching-case]
sources: [s-tool-rkn-block-checker]
updated: 2026-05-17
---

# DNS AAAA cascade failure — investigation teaching case

This is a **synthesis page**, not a concept page. It documents a
single-observation investigation whose mechanism hypothesis was
later found inconsistent with the system's actual routing state.
The wiki preserves it as a methodology teaching case. **Other pages
should not link to this as a citable mechanism.**

The page was originally filed under `concepts/` based on a single
observation. It was moved to `synthesis/` after the correction;
per schema, single-observation failures with unconfirmed
mechanisms don't belong in `concepts/`.

## Status

- **Original mechanism hypothesis** (TSPU AAAA-drop on `8.8.8.8`
  cascading through macOS libc into `getaddrinfo` failure):
  **most-likely invalidated**, conditional on continuous TUN
  auto-route through the original observation window. The
  invalidation rests on a post-hoc routing-state check; a brief
  TUN-down or auto-route-not-yet-installed window at observation
  time would re-open the original interpretation.
- **Symptom** (transient `gaierror` on `www.vtb.ru`): real,
  single-observation, did not reproduce on re-verification, no
  confirmed mechanism.
- **Page role**: methodology teaching case (synthesis, not concept).
  Other pages should not link to this as a citable mechanism.

## What was observed (the facts)

On a RU-consumer-network vantage Mac on 2026-05-17, an `rkn-check`
run returned `✗ DNS — system DNS doesn't resolve, DoH does` for
`www.vtb.ru`. Concurrent diagnostic commands:

- `dig +short www.vtb.ru` (default-resolver path) → `195.242.83.13`.
- `dig +short @8.8.8.8 AAAA www.vtb.ru` (explicit upstream) →
  timed out on the first attempt; subsequent retries returned clean
  NOERROR-no-answer.
- `python3 socket.getaddrinfo("www.vtb.ru", None, family=AF_INET)`
  → `gaierror`.
- `curl https://www.vtb.ru/` → "Could not resolve host".
- `dscacheutil -flushcache`, retry → still failed.

**Important: under the corrected baseline (next section), the
`dig @8.8.8.8 AAAA` packet was TUN-intercepted by sing-box and
answered by sing-box's internal DNS path (russia-DoH to
`dns.comss.one` for `.ru` hosts), NOT by the literal `8.8.8.8`
host named in the `@` argument.** Same for the default-resolver
`dig` — it used the system's configured resolvers (1.1.1.1 /
8.8.8.8 / 8.8.4.4) which were also TUN-intercepted. The "raw
A-query bypassing libc resolver" framing in the original page was
wrong: `dig` bypasses libc, but the *packets* still traverse the
same TUN. What `dig @8.8.8.8 AAAA` saw was sing-box's internal
path, not TSPU-on-8.8.8.8.

## Corrected baseline (the post-hoc check that flipped the story)

Hours later, a routing verification on the same Mac:

- `pgrep -fl sing-box` → PID 45707 active.
- `route -n get 1.1.1.1` → interface `utun7` (the sing-box TUN).
- `route -n get 8.8.8.8` → interface `utun7`.
- `route -n get <dns.comss.one IP>` → interface `utun7`.

The TUN auto-route was intercepting all DNS traffic at the time of
the check. **Critical caveat:** this check was *post-hoc*. It does
not directly prove the same state was uninterruptedly active during
the original symptom window. A brief sing-box restart, route flap,
or auto-route-not-yet-installed window at observation time could
re-open the original interpretation. The page treats invalidation as
**conditional on continuous TUN auto-route through the observation
window** — not absolute.

A re-verification at correction time:

- `getaddrinfo("www.vtb.ru", AF_INET)` clean 5/5.
- `rkn-check` verdict flipped to `✓ OK`.
- sing-box log: clean `dns: exchanged A www.vtb.ru. ... IN A
  195.242.83.13`.

The symptom did not reproduce. Symptom non-reproduction is a
*weaker* signal than the routing fact — a transient adversary
behavior could produce the same evidence. The mechanism's
invalidation rests on the routing fact (Lesson 1 below), not on the
symptom going away.

## Why the original mechanism is most-likely invalidated

The original page proposed: "macOS libc `getaddrinfo(AF_INET)` issues
parallel AAAA; TSPU drops AAAA queries to 8.8.8.8 selectively for
banking hostnames; the AAAA timeout cascades into a full
`getaddrinfo` failure." That story rested on an unstated assumption:
the libc resolver was talking directly to `8.8.8.8`.

Under the corrected baseline, no packet to `8.8.8.8` is generated:
TUN auto-route catches every DNS-bound packet at L3 and hands it to
sing-box's internal DNS server, which applies its rules and chooses
its own upstream. For `www.vtb.ru` (matches `.ru` suffix), the
upstream is `russia-dns` (DoH to `dns.comss.one`). 8.8.8.8 was
never in the packet path.

Conditional on continuous TUN auto-route, "TSPU drops AAAA at
8.8.8.8" cannot be the mechanism. The page treats the hypothesis as
**most-likely invalidated**, not absolutely refuted, because the
continuity assumption is itself an inference from one post-hoc
snapshot.

## What this leaves us with (open questions, not new mechanisms)

The symptom is real and unattributed. *Possible* failure modes
consistent with the corrected baseline — listed as open questions,
not as endorsed candidate mechanisms (the original page introduced a
"candidate mechanisms" table with a research-agenda column; that
was doing fresh concept work under a teaching-case banner, exactly
the mistake the page is meant to teach against):

- Did sing-box's resolution of `www.vtb.ru` (via `russia-dns` DoH —
  A and/or AAAA) return a transient failure (SERVFAIL, timeout, or
  unexpected upstream behavior)? Investigate with sing-box
  `level: debug` log.
- Did mDNSResponder or some macOS-internal libc cache layer hold a
  negative entry that `dscacheutil -flushcache` did not reach?
  Investigate with `sudo killall mDNSResponder`.
- Did a brief sing-box restart / route-not-yet-installed window
  coincide with the symptom? Investigate by correlating sing-box's
  launchd start time with the symptom timestamp.
- Was there a transient packet drop between libc and sing-box's
  internal DNS server (`172.19.0.1`) during the observation?

Note: each of these covers BOTH A and AAAA paths. The original
page's mistake was binding the open question to AAAA specifically
without evidence that libc's failed call had an AAAA leg involved
at all. The libc `gaierror` is just "lookup failed" — it does not
itself constrain which record type the resolver chain was working
on.

## Lessons (the actual reason this page is kept)

1. **Verify baseline routing before attributing a symptom to a
   mechanism.** The original observation conflated "system resolver
   configured at 1.1.1.1" with "system resolver reaching 1.1.1.1".
   TUN auto-route makes those two different. A 30-second
   `route -n get <resolver>` check would have rescued the
   investigation from the start.

2. **Don't codify a single observation as a named mechanism.** One
   gaierror on one host at one moment is not a mechanism. The
   Karpathy-pattern wiki preserves uncertainty by design — single
   observations belong in a log entry or a synthesis-page open
   question, not in a `concepts/` page with a stable slug other
   pages link to. This page itself was filed under `concepts/`
   initially; the move to `synthesis/` is the correction.

3. **Symptom non-reproduction ≠ mechanism invalidation.** What
   most-likely invalidated this mechanism was the routing check
   (Lesson 1), which showed the proposed cause was structurally
   inaccessible — conditional on the routing state having been
   continuous through the observation window.
   Re-verification of the symptom going away is a weaker signal —
   a transient adversary behavior produces the same evidence.
   Always pair symptom-re-verification with a structural check.

## Operational guidance (independent of the most-likely-invalidated hypothesis)

If you see a "dig works, apps don't" symptom again on this fleet:

1. **First**: `route -n get 8.8.8.8` (and `1.1.1.1`) on the affected
   Mac. If they point at `utun*`, sing-box is intercepting — the
   query never reaches the listed resolver. Investigate sing-box's
   DNS path, not the public resolver.
2. **Second**: `curl -s "http://127.0.0.1:9090/dns/query?name=<host>&type=A"`
   and `&type=AAAA` to see what sing-box's internal resolver returns
   per record type. If clean, the failure is between sing-box and
   libc.
3. **Third**: `sudo killall mDNSResponder` for a full cache restart
   (more reliable than `dscacheutil -flushcache`).
4. **Fourth**: only after the above, consider raising sing-box log
   level to `debug` (requires config edit + restart, which costs a
   reconnect window).

## Sources

- [[s-tool-rkn-block-checker]] — the tool whose `✗ DNS` verdict
  surfaced the original symptom and whose author's README
  classifies "DNS poisoning" as a possible explanation among
  several.
- First-party investigation and routing-verification, 2026-05-17,
  RU consumer vantage with sing-box TUN auto-route active.

(The original page listed `s-2026-05-tspu-asn-camouflage-research`
in `sources:` as evidence-grounding for the TSPU-on-8.8.8.8
mechanism. Under the corrected baseline TSPU was never in the
packet path for this observation; that source has been dropped from
this page's frontmatter and Sources list.)
