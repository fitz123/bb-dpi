# ASN-match SNI camouflage

The strategy: choose REALITY's `dest` so the hostname's resolved IP
lives on the **same AS** (autonomous system) as the REALITY server's
own IP. This blocks the most-common passive correlation attack against
[[reality-protocol]].

## The attack it defeats

REALITY's active-probe resistance is strong, but a passive observer
can sometimes guess what's happening at the network layer alone, no
probing needed:

> "This IP (the REALITY server) claims via TLS SNI to be hosting
> `<dest-hostname>`. But `<dest-hostname>` is actually hosted at a
> completely different IP, on a completely different network operator.
> That's a mismatch — flag the IP."

If `dest` is a CDN hostname on cloud provider A and the REALITY server
is on cloud provider B, the observer sees a provider-B IP serving SNI
for a provider-A-CDN hostname. A simple ASN-correlation pipeline
catches that.

ASN-match makes the SNI choice consistent with the server's own
network: a REALITY server on provider X uses an SNI hostname whose
authoritative IPs are *also* on provider X. The mismatch disappears.

## How to apply it

For each REALITY server, the deploy-time choice of `dest` /
`serverNames` follows this checklist:

1. Look up the server's own IP's ASN (`whois`, `bgp.tools`, or
   `team-cymru-IP-to-ASN`).
2. Find a real, busy public hostname whose authoritative IPs (or CDN
   pool) live on the same AS.
3. Verify the candidate hostname accepts vanilla TLS ClientHellos
   (some CDNs front their origin with a WAF that rejects unmodified
   handshakes — those are unusable; see [[reality-protocol#operational-notes]]).
4. Confirm with `openssl s_client -connect <server-ip>:443
   -servername <candidate-host>` from a vantage point that doesn't
   know the REALITY key — the response must be `<candidate-host>`'s
   real cert chain, not a synthetic REALITY cert.

## Per-role guidance

- **Direct exit on a cloud-provider AS** → an object-store-style or
  region-plausible CDN hostname on the *same* cloud provider. Region
  match matters: a us-west exit shouldn't claim to be an eu-north
  endpoint.
- **Direct exit on a hosting provider's AS** → empirically, sometimes
  the obvious provider-family SNI on the matching AS still breaks
  (see [[empirical-sni-failures]]). Be prepared to fall back to a
  known-good but non-ASN-matched hostname if the strict-match attempt
  causes outages.
- **Relay on an in-region (consumer ASN-adjacent) datacenter** → a
  CDN-CNAMED consumer-site hostname on the same ASN. The relay's
  first-hop traffic from a consumer ISP to a consumer-site CDN on the
  same provider is the most plausible-looking flow available.

## Literal, not wildcard

`serverNames` should be a single literal hostname. Wildcards
(`*.example.com`) are not a supported camouflage primitive in
REALITY — the active-probe forwarding goes to `dest`, not to a wildcard
match, so the camouflage is only as good as the literal `dest` value.
Codex's review on a related plan confirmed this rule explicitly.

## Limits of ASN-match

ASN-match defeats the *coarse* SNI/IP correlation attack. It does NOT
defeat:

- **Per-prefix or per-cluster heuristics.** Real CDN traffic for
  `<host>` may resolve to a specific /24 cluster within the AS, not
  the whole AS. A REALITY server on a different /24 in the same AS
  still shows up as "wrong cluster".
- **TLS-layer fingerprint mismatch.** See [[utls-fingerprint-staleness]].
- **Latency-delta probing.** See [[latency-delta-active-probe]].
- **Cross-flow DNS↔TLS correlation.** A passive observer who sees
  *both* the client's plaintext DNS for `<host>` AND the client's
  TLS connection to a non-pool IP with `SNI=<host>` can join them.
  Mitigated by [[doh-for-split-routed-dns]].

ASN-match is a necessary but not sufficient layer.

## Empirical caveats

The strategy is documented; the practice has edge cases. Trying to
ASN-match a server whose provider's CDN-family SNIs are themselves on
a censor's hot-list can be *worse* than using an empirically-stable
but non-matched SNI. The project's history records one such case (see
[[s-memory-sni-asn-correlation-incident]] line 41-47): a strict ASN-match attempt
caused unpredictable wedges that took hours to roll back. The
operative rule: ASN-match by default, but accept empirical evidence
when it diverges.

## Sources

- [[s-memory-sni-asn-correlation-incident]]
- [[s-memory-chain-relay-rationale]]
