# Source: project memory — RU DPI / cloud-AS SNI correlation

Out-of-repo memory file documenting observed RU consumer ISP DPI
behavior in 2026-05, specifically tied to a (SNI, destination ASN)
correlation drop pattern and the time-windowed flow-learning trigger
that compounded it.

- **Type**: out-of-repo project memory (not citeable by path; lives in
  the operator's local `memory/` directory)
- **Subject**: a 2026-05-13 incident debugging dead-tunnel symptoms on
  a cloud-region exit
- **Ingested**: 2026-05-14 (DPI-research learnings extracted to wiki;
  PII held back to memory)

## What it documents

### The SNI/IP correlation pattern

A direct exit on a major cloud provider's network, running REALITY
with a popular non-ASN-matched generic CDN SNI on the TCP+vision
inbound, was **selectively dropped by RU consumer ISP DPI**:

- TCP handshake completes normally (`nc -vz` shows green).
- Application-data PSH packets carrying the REALITY+vision handshake
  payload **never arrive at the server NIC**. Two-sided tcpdump
  showed PSHs leaving the client's external interface and zero
  arriving on the server's NIC at the same wall-clock time.
- The drop is keyed on (SNI, destination ASN) tuples — same SNI to a
  non-cloud exit works fine; same cloud exit with a different
  (ASN-appropriate) SNI works fine.
- XHTTP on the SAME server (port 443, same REALITY identity, different
  framing) was unaffected.

This is the canonical SNI/IP correlation attack
[[asn-match-sni-camouflage]] is designed to defeat — and confirms it's
deployed in RU consumer ISP DPI today.

### Time-windowed flow-learning trigger

Beyond the static (SNI, ASN) correlation, the memory documents a
**dynamic** layer: the DPI builds time-windowed flow-blocks triggered
by sustained probe-failure patterns:

- Swapping the burnt exit to an ASN-plausible SNI (object-store-
  style hostname matching the actual cloud region) reverses the drop.
- **But**: ~30 minutes later, PSH packets start dropping again.
- A parallel sandbox exit on a different port + same new SNI keeps
  working.
- ~3 hours later, the original (now-stuck) flow starts working again
  with no operator action.

Conclusion documented in the memory: the trigger is *the probe-
failure pattern*, not the SNI alone. Once urltest's 30s-interval
probes accumulate ~5-10 failures against a 5-tuple, the heuristic
fires; after the burn decays (hours), the same SNI/IP/port works
again.

This is the foundation of [[dpi-flow-learning]].

### Operational rules derived

- **For new cloud-hosted REALITY exits**: use a region-plausible SNI
  hostname on the actual hosting cloud's ASN (e.g., a regional
  object-store endpoint for the same cloud). Don't use generic
  non-ASN-matched CDN hostnames on cloud exits.
- **For non-cloud exits**: empirical evidence beats heuristic — the
  hosting-provider-family SNI for one specific exit caused
  unpredictable wedges that were painful to roll back. Don't retry
  it without a structural reason.
- **Avoid kickstarting xray-xhttp unnecessarily**: each kickstart
  causes a burst of urltest probe failures across all `xhttp-*`
  outbounds during the restart window. Empirically this has burnt
  separate healthy flows on the same client.
- **Render upstream-only servers with `client_render: false`** so
  clients don't directly probe them and contribute to the
  probe-failure burst that triggers flow-learning.
- **For RU paths, prefer `--proto xhttp`**: TCP+vision is more
  recognizable; XHTTP framing is more resilient.

## Diagnostic technique used

The diagnosis was only possible via [[two-sided-tcpdump-diagnostic]] —
the asymmetric pattern (PSHs left client, zero arrived server) is
*uniquely* diagnostic for in-path payload-aware drops.

## Touched concept pages

- [[dpi-flow-learning]] — primary source for the time-windowed pattern
- [[asn-match-sni-camouflage]] — primary source for the correlation
  attack
- [[two-sided-tcpdump-diagnostic]] — the technique that surfaced this
- [[xhttp-transport]] — XHTTP-vs-TCP+vision resilience comparison
- [[reality-protocol]] — incident context

## Caveats

- Single-fleet observation. The patterns may differ in other RU
  consumer ISPs (different middlebox vendors), different countries,
  or as RU DPI evolves.
- The flow-learning *decay window* (1-3 hours) is rough — measured
  twice, not a formal study.
- The "don't retry the provider-family SNI" rule on one specific
  non-cloud exit is empirical and the underlying classifier is
  unknown (likely a per-prefix or per-cert-family list rather than
  pure ASN).
