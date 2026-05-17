# Two-sided tcpdump for dead-tunnel diagnosis

A diagnostic technique. When TCP handshakes succeed but data doesn't
flow — the **dead-tunnel** pattern — single-sided packet captures
cannot reliably distinguish three different failure modes that
present identically at the application layer.

The fix: capture on **both** endpoints simultaneously at the same
wall-clock time, then compare patterns to localize the drop.

## The problem

A user-visible symptom of "VPN is up but I can't reach anything"
maps to several distinct root causes:

| Failure mode | What's happening |
|---|---|
| **In-path payload-aware drop** | A middlebox (DPI / corporate proxy / ISP filter) is dropping application-data packets but letting TCP control bits through. TCP looks healthy from each endpoint's local view. |
| **App-layer hang** (server) | Server received data, didn't respond. Could be a stuck thread, a config error, a fallback target unreachable from the server. |
| **App-layer hang** (client) | Client thinks it's writing but the write got stuck somewhere in the local stack. Local-only, but symptom is identical to network drop. |

ICMP pings and bare `nc -vz <host> <port>` are no help — they only
exercise the *control plane*. A dead-tunnel host will show as "fine"
on ICMP and on bare TCP-connect probes, while every actual HTTPS
request times out.

## The technique

Run two simultaneous packet captures with synchronized wall-clock
references:

**Client (any test client on the affected vantage):**
```bash
sudo timeout N tcpdump -i en0 -n 'tcp and host <SERVER_IP> and port <PORT>'
```

**Server:**
```bash
sudo timeout N tcpdump -i ens5 -n 'tcp port <PORT> and host <CLIENT_PUBLIC_IP>'
```

Trigger the failure (`curl https://1.1.1.1` from the client) while
both captures run. Filter to PSH-only with
`'tcp[tcpflags] & tcp-push != 0'` after capture to cut through the
FIN noise from previous-session teardowns.

## Decision matrix

Compare PSH patterns from the two captures:

| Client-side view | Server-side view | Diagnosis |
|---|---|---|
| Many PSH out, retransmits, no ACK | **Zero PSH from client** | In-path **payload-aware drop**. Look for SNI/IP correlation, payload-aware classifier. See [[dpi-flow-learning]]. |
| PSH out, arrives both ways, no app response | PSH arrives, no response written back | **App-layer hang on server**. Check service health, config validity, fallback target reachability. |
| No PSH from client at all | Only SYN/ACK | **Client-side issue**. Sing-box not writing — TFO state stuck, urltest blocked, connection pool corrupted. Trace by session-id in the client log. |
| TCP handshakes also fail (SYN-ACK retransmits) | (depends) | **Middlebox treating TCP options suspiciously.** Try `tcp_fast_open: false` on outbound — TFO `cookiereq` is a common trigger. Independent of the PSH-drop class above. |

The asymmetry — PSHs left the client interface but never arrived on
the server — is *uniquely diagnostic* for in-path payload-aware drops.
No single-sided capture can distinguish that from a server hang.

## Why this matters

The project's history records ~2 hours lost on one incident chasing
single-sided red herrings (MTU, TFO, MSS) before two-sided capture
revealed the actual cause was a payload-aware DPI drop tied to a
specific (SNI, destination-ASN) tuple — see
[[s-memory-twosided-tcpdump]] line 12 and [[s-memory-sni-asn-correlation-incident]]
for the specific 2026-05-13 incident.

The technique has since been codified as the standing operational
rule for any dead-tunnel diagnosis.

## Operational notes

- **Synchronize wall clocks.** Even ~1 second of drift makes side-by-
  side comparison harder. If the two hosts can't sync via NTP cleanly,
  trigger a known reference packet first (`curl http://1.1.1.1` —
  port 80 packet visible to both) and align the captures by that
  reference.
- **Pick `N` (timeout) so you get 5-10 PSH events.** Too short and
  you miss the pattern; too long and the output is noise.
- **PSH-only filter is essential** to skip FIN/ACK noise from
  previously-torn-down sessions.

## When to escalate beyond this technique

Two-sided tcpdump tells you *where* the drop happens. It does not tell
you *why* the censor classified the flow as drop-worthy. For that:

- Compare wire patterns between a working flow and the burnt flow on
  the same client → look for the discriminator.
- Test alternate SNIs on the same server-IP — does swapping
  `dest`/`serverNames` to a different hostname change the drop
  behavior? If yes, SNI is in the classifier. See
  [[asn-match-sni-camouflage]].
- Test the same SNI on a different server-IP — same question for IP.
  If both matter, you're looking at a correlation, not a single-axis
  classifier.

## Before reaching for tcpdump

Two-sided tcpdump is a heavy diagnostic — synchronized captures on two
hosts, post-filtering for PSH patterns, decision-matrix interpretation.
For *first-triage* of "site stopped opening", the lighter tool is
[[s-tool-rkn-block-checker]]: it probes a client vantage's DNS/TCP/TLS/
HTTP stack per-target and classifies failures by signal type (TLS reset
after ClientHello → SNI-DPI; silent drop after handshake → flow-burn;
sys/DoH DNS disjoint → DNS poisoning). Use it to *eliminate* the broad
classes before committing to a two-sided capture session.

The tools are complementary, not interchangeable. rkn-check classifies
per-target by wire signal; two-sided tcpdump localizes the drop point
on a *specific known-affected* tunnel.

## Sources

- [[s-memory-twosided-tcpdump]] — primary source for the technique
  and the incident that motivated codifying it.
- [[s-memory-sni-asn-correlation-incident]] — the specific dead-tunnel incident
  this diagnosed.
- [[s-tool-rkn-block-checker]] — lighter-weight first-triage tool
