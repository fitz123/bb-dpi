# REALITY protocol

REALITY is XTLS's anti-active-probe TLS-on-the-wire scheme. A REALITY
server speaks to legitimate clients with TLS that LOOKS like a TLS
reverse-proxy of some real third-party site (`dest`), while forwarding
unauthenticated probes to that real `dest` so the server is
indistinguishable from a fronting proxy without the REALITY key.

## How it works (short version)

1. Client opens a TCP connection to `<reality-server>:443`.
2. Client sends a TLS ClientHello with `SNI = <dest-hostname>` and an
   embedded REALITY auth blob in a TLS extension.
3. Server reads the ClientHello.
   - If the REALITY auth signature is valid → server completes the
     handshake using its own REALITY identity. The peer is now a
     legitimate REALITY client.
   - If invalid (or absent) → server opens its own TCP connection to
     `dest:port` and forwards bytes back and forth. The probe sees
     `dest`'s real cert chain and behaves identically to a probe
     directly against `dest`. (xray-core's `MirrorConn` implements
     this forwarding loop.)

The auth blob rides in the otherwise-padded portion of the ClientHello,
so a passive observer can't distinguish a REALITY ClientHello from a
plain TLS ClientHello to `dest`. An active prober probing the
REALITY-server IP gets `dest`'s real cert and behavior, so the server
*looks* like a legitimate reverse proxy of `dest`.

## Config surface (xray-core)

Server-side inbound:
```jsonc
"streamSettings": {
  "security": "reality",
  "realitySettings": {
    "dest": "<dest-hostname>:<port>",
    "serverNames": ["<dest-hostname>"],
    "privateKey": "<x25519-priv>",
    "shortIds": ["<8-byte-hex>"],
    "show": false        // suppress per-handshake debug logs in prod
    // xver: 0           // default, never set ≥1 — see below
  }
}
```

Client-side outbound mirrors the SNI + `publicKey` + `shortId` +
`fingerprint` ("chrome" via uTLS).

`serverNames` is an array of accepted SNI values per inbound. Best
practice is a single literal hostname matching `dest` — wildcards are
not supported in a useful way (see [[asn-match-sni-camouflage#literal-not-wildcard]]).

`shortIds` is a server-side array; clients pin one. The array allows
brief overlap during rotation if rotation ever happens (see
[[reality-key-rotation-gap]]).

## `xver: 0` is load-bearing

Never set `xver` to 1 (PROXY protocol v1) or 2 (v2) on a REALITY
inbound. With non-zero `xver`, xray sends a PROXY-protocol header to
the `dest` server containing the client's source IP. The `dest` server
is third-party (CDN, cloud object store) — leaking client IPs to it is
both an information leak AND a fingerprintable behavior. Default is
0 (no PROXY protocol); the project's standing deploy validation
explicitly greps for any inbound carrying an `xver` key.

## Why active probing is defeated

REALITY's design defeats the canonical active-probe attack: a censor
sees a suspicious-looking TLS endpoint, probes it directly with
their own TLS ClientHello, and checks whether the response matches a
real-website fingerprint. With REALITY:

- The probe's bytes are forwarded raw to the real `dest`.
- The real `dest`'s TLS response (cert chain, ServerHello extensions,
  ALPN, cipher selection) is what the prober sees.
- The probe is indistinguishable from a probe of `dest` itself.

The censor cannot distinguish "real reverse proxy of `dest`" from
"REALITY server claiming to be a reverse proxy of `dest`" via active
probing alone.

## What REALITY does NOT defend against

- **Latency-delta probing.** See [[latency-delta-active-probe]].
  A probe to a REALITY server adds the `server → dest` RTT on top of
  the `prober → server` RTT. A direct probe to `dest` doesn't.
  The delta is measurable at scale.
- **SNI / IP correlation.** See [[asn-match-sni-camouflage]] for why
  the chosen `dest` must be plausibly hosted in the same network as
  the REALITY server.
- **Cross-flow correlation via DNS.** If the client's plaintext DNS
  shows queries for `dest`'s real CDN pool while the wire shows TLS
  to a non-CDN IP carrying `SNI = dest`, the join is observable.
  Mitigated by DoH for split-routed regional DNS — see
  [[doh-for-split-routed-dns]].
- **TLS-layer fingerprint mismatch.** REALITY relies on uTLS to
  produce a real-Chrome ClientHello shape; staleness vs. real
  Chrome's evolving JA4 is observable. See [[utls-fingerprint-staleness]].
- **TCP-layer fingerprint divergence.** A REALITY-tunneled SYN with
  TCP options that don't match what real Chrome sends from the same
  OS is a discriminator. See [[tcp-option-coherence]].

## Operational notes

- The `dest` must accept *vanilla* TLS ClientHellos without WAF
  interference. Some CDNs (especially WAF-fronted edges with strict
  fingerprint-based filtering) reject unmodified ClientHellos,
  making them unusable as REALITY `dest`. Probe candidates with
  `openssl s_client` before adopting.
- The `dest` should ideally be reachable from the REALITY server's
  network at very low RTT — same datacenter or same regional CDN
  pop — to minimize the latency-delta side channel.
- `dest`-IP changes silently. The REALITY server resolves `dest` via
  the host's resolver; a `docker compose up -d` recreate may leave
  the old in-process resolver state behind. Explicit `docker compose
  restart xray` after any dest change is part of the deploy contract
  (codified in [[s-memory-chain-relay-rationale]]).

## New attack surfaces (2025-2026)

- **Port 443 selection-bias**: identical VLESS+REALITY on 443 hits
  TSPU inspection at much higher rate than on high ports (47000+);
  ~80% pass-through reported on high ports vs near-zero on 443
  ([[s-2026-05-tspu-asn-camouflage-research]]). REALITY's
  port-443 default no longer assumes blending with normal HTTPS;
  it now means *maximum inspection sampling*. Alternative-port
  REALITY inbounds are an underused mitigation, at the cost of
  client-side port-knock complexity.
- **AI-keyed REALITY detector** (Dec 2025): RKN reportedly deployed
  an ML detector keyed on TLS-1.3 handshake patterns to
  non-whitelisted foreign IPs. Detector is destination-aware and
  applies to both v4 and v6. xray-core 25.12.8+ adds `testpre` /
  `testseed` defenses against the timing+entropy ML signal — pin
  minimum version on both ends of a chain.
- **xver re-confirmed as load-bearing** by RKN's destination-side
  heuristics: leaking client IPs via PROXY protocol to the third-
  party `dest` is now an even bigger discriminator than before.

## Sources

- [[s-memory-chain-relay-rationale]] — operational notes (dest-change restart, ASN-match `dest` choice)
- [[s-2026-05-tspu-asn-camouflage-research]] — port-443 selection-bias, SNI/IP correspondence
- [[s-2026-05-xray-relay-community-reports]] — Dec 2025 AI-keyed REALITY detector, xray-core 25.12.8+ countermeasures
- xray-core source: `XTLS/REALITY/tls.go` (`MirrorConn`)
- Xray docs: [REALITY transport reference](https://xtls.github.io/en/config/transports/reality.html)
