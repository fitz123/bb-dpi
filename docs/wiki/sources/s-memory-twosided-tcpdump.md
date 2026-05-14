# Source: project memory — two-sided tcpdump for dead-tunnel diagnosis

Out-of-repo memory file codifying a diagnostic technique: when TCP
handshakes succeed but data doesn't flow, capture packets on BOTH
endpoints simultaneously at the same wall-clock time and compare PSH
patterns to localize the drop.

- **Type**: out-of-repo project memory
- **Ingested**: 2026-05-14
- **Subject**: diagnostic methodology for the "dead tunnel" failure
  class

## What it captures

The full technique is documented in the concept page:
[[two-sided-tcpdump-diagnostic]].

This source page records the *origin story* and *operational
constraints* that the concept page doesn't:

### Origin

The technique was codified after a 2-hour debugging session chased
single-sided red herrings (MTU, TFO, MSS) before two-sided capture
revealed the actual cause was a payload-aware in-path drop tied to
SNI/IP correlation ([[s-memory-sni-asn-correlation-incident]]).

The lesson: ICMP ping and `nc -vz` only exercise the *control plane*.
A flow that's TCP-up but data-dropping shows green on both.
Single-sided packet capture can't distinguish in-path drop from
server-app hang from client-app hang — they all look the same locally.

Two-sided capture is the *only* signal that uniquely diagnoses
in-path payload-aware drop, by the asymmetry: "PSHs left the client
interface, zero arrived on the server".

### Operational constraints

- **Wall-clock sync matters.** ~1 second of drift makes side-by-side
  comparison harder. If NTP isn't tight, use a known reference
  packet (`curl http://1.1.1.1` — port 80 visible to both) to align
  the captures.
- **PSH-only filter is essential**, otherwise the output drowns in
  FIN/ACK noise from prior teardowns.
- **Pick the timeout so you get 5-10 PSH events.** Too short = miss
  the pattern; too long = noise.

### Decision matrix shipped in the memory

| Client view | Server view | Diagnosis |
|---|---|---|
| Many PSH out, retransmits, no ACK | Zero PSH from client | In-path payload-aware drop |
| PSH out, arrives both ways, no app response | PSH arrives, no response | App-layer hang on server |
| No PSH from client | Only SYN/ACK | Client-side issue (sing-box stuck, TFO state, etc.) |
| TCP handshakes also fail (SYN-ACK retransmits) | (depends) | Middlebox treating TCP options suspiciously — try `tcp_fast_open: false` |

The last row is independent of the PSH-drop class above — TCP options
(notably TFO `cookiereq`) are themselves a fingerprint that some
middleboxes treat as suspicious. The project disables TFO on its
TCP+vision outbound for exactly this reason.

## Touched concept pages

- [[two-sided-tcpdump-diagnostic]] — primary
- [[dpi-flow-learning]] — the failure class this technique diagnoses
- [[reality-protocol]] — context (dead-tunnel under REALITY transport)

## Why this is in the wiki vs. just a memory file

The technique applies broadly — anyone debugging REALITY-tunneled
traffic against an opaque DPI middlebox will hit dead-tunnel patterns
and need to localize them. It's *generalisable knowledge*, not a
project-internal workflow rule. Worth the wiki page.
